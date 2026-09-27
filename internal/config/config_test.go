package config

import (
	"os"
	"testing"
	"time"
)

// clearEnv removes every AGENTFORGE_* variable for the duration of a test.
// Without this, a developer machine leaks state into the suite and the tests
// pass locally while failing in CI.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"AGENTFORGE_ADDR", "AGENTFORGE_DATA_DIR", "AGENTFORGE_PROVIDER",
		"AGENTFORGE_MODEL", "AGENTFORGE_KEY_ENV", "AGENTFORGE_TIMEOUT",
		"GEMINI_API_KEY", "GROQ_API_KEY",
	} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
}

func TestDefaultsAreValid(t *testing.T) {
	clearEnv(t)
	cfg := Defaults()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("default config invalid: %v", err)
	}
	// Loopback is a safety property, not a preference.
	if cfg.Addr != "127.0.0.1:8787" {
		t.Errorf("default addr = %q, want loopback 127.0.0.1:8787", cfg.Addr)
	}
}

func TestEchoNeedsNoKeyOrModel(t *testing.T) {
	clearEnv(t)
	cfg, err := Load(map[string]string{"Provider": "echo"})
	if err != nil {
		t.Fatalf("echo config rejected: %v", err)
	}
	if cfg.Model != "" {
		t.Errorf("echo model = %q, want empty", cfg.Model)
	}
}

func TestUnknownProviderRejected(t *testing.T) {
	clearEnv(t)
	if _, err := Load(map[string]string{"Provider": "openai"}); err == nil {
		t.Fatal("expected an error for an unknown provider")
	}
}

// This is the regression test for the bug where `--provider groq` still read
// GEMINI_API_KEY and produced a 401 that looked like an invalid key.
func TestProviderSwitchAdoptsThatProvidersDefaults(t *testing.T) {
	clearEnv(t)

	cfg, err := Load(map[string]string{"Provider": "groq"})
	if err != nil {
		t.Fatalf("groq config rejected: %v", err)
	}
	if cfg.APIKeyEnv != "GROQ_API_KEY" {
		t.Errorf("APIKeyEnv = %q, want GROQ_API_KEY", cfg.APIKeyEnv)
	}
	if cfg.Model != "openai/gpt-oss-120b" {
		t.Errorf("Model = %q, want openai/gpt-oss-120b", cfg.Model)
	}
}

func TestExplicitModelBeatsProviderDefault(t *testing.T) {
	clearEnv(t)
	cfg, err := Load(map[string]string{"Provider": "groq", "Model": "llama-3.3-70b-versatile"})
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if cfg.Model != "llama-3.3-70b-versatile" {
		t.Errorf("Model = %q, explicit model was overwritten", cfg.Model)
	}
}

func TestEnvIsReadAndFlagWins(t *testing.T) {
	clearEnv(t)
	t.Setenv("AGENTFORGE_ADDR", "0.0.0.0:9999")
	t.Setenv("AGENTFORGE_PROVIDER", "groq")

	// Env alone.
	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("load from env failed: %v", err)
	}
	if cfg.Addr != "0.0.0.0:9999" {
		t.Errorf("Addr = %q, want the env value", cfg.Addr)
	}
	if cfg.APIKeyEnv != "GROQ_API_KEY" {
		t.Errorf("APIKeyEnv = %q, env provider should switch the key var", cfg.APIKeyEnv)
	}
	if src := cfg.Source["Addr"]; src != "env AGENTFORGE_ADDR" {
		t.Errorf("Addr source = %q, want the env to be recorded", src)
	}

	// Flag beats env.
	cfg2, err := Load(map[string]string{"Addr": "127.0.0.1:1234"})
	if err != nil {
		t.Fatalf("load with override failed: %v", err)
	}
	if cfg2.Addr != "127.0.0.1:1234" {
		t.Errorf("Addr = %q, flag should win over env", cfg2.Addr)
	}
	if src := cfg2.Source["Addr"]; src != "flag" {
		t.Errorf("Addr source = %q, want flag", src)
	}
}

func TestTimeoutParsing(t *testing.T) {
	clearEnv(t)
	cfg, err := Load(map[string]string{"Timeout": "90s"})
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if cfg.Timeout != 90*time.Second {
		t.Errorf("Timeout = %s, want 90s", cfg.Timeout)
	}
	if _, err := Load(map[string]string{"Timeout": "soon"}); err == nil {
		t.Fatal("expected an error for an unparseable timeout")
	}
}

// The key is read at call time, not load time, so a key added after start-up
// works without a restart.
func TestAPIKeyIsReadLate(t *testing.T) {
	clearEnv(t)
	cfg, err := Load(map[string]string{"Provider": "gemini"})
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if cfg.APIKey() != "" {
		t.Fatal("expected no key before it is set")
	}
	t.Setenv("GEMINI_API_KEY", "secret-value")
	if got := cfg.APIKey(); got != "secret-value" {
		t.Errorf("APIKey() = %q, want the newly set value", got)
	}
}

func TestDatabasePathJoinsDataDir(t *testing.T) {
	clearEnv(t)
	cfg, err := Load(map[string]string{"Provider": "echo", "DataDir": "/tmp/af-test"})
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if got, want := cfg.DatabasePath(), "/tmp/af-test/agentforge.db"; got != want {
		t.Errorf("DatabasePath() = %q, want %q", got, want)
	}
}

// A zero or negative timeout would let a request hang forever, which on a
// phone looks like the app being stuck.
func TestNonPositiveTimeoutRejected(t *testing.T) {
	clearEnv(t)
	cfg := Defaults()
	cfg.Timeout = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected an error for a zero timeout")
	}
}
