// Package config reads AgentForge's settings.
//
// The design goal is that a phone user never has to write a config file by
// hand. Everything here is either a flag, an environment variable, or a
// sensible default, and `doctor` reports which source each value came from.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config is the resolved runtime configuration.
type Config struct {
	// Addr is the listen address for the local HTTP server.
	// Loopback by default: an agent server reachable from the whole network
	// with no auth is a liability, not a feature.
	Addr string

	// DataDir holds the SQLite file and any other persistent state.
	DataDir string

	// Provider selects the model backend: "gemini", "groq", or "echo".
	Provider string

	// Model is the provider-specific model id.
	Model string

	// APIKeyEnv names the environment variable holding the provider key.
	// The key itself is never read from the config file, so the config file
	// stays safe to commit.
	APIKeyEnv string

	// Timeout bounds a single provider request.
	Timeout time.Duration

	// Source records where each key came from, for `doctor` output.
	Source map[string]string `json:"-"`
}

// providerDefaults maps each backend to the key variable it reads and the
// model used when none is given. Keeping this in one table means
// `--provider groq` automatically stops reading GEMINI_API_KEY, which was a
// real bug: the switch happened but the key variable did not, producing a
// 401 that looked like a bad key rather than a wrong variable.
var providerDefaults = map[string]struct{ KeyEnv, Model string }{
	"gemini": {"GEMINI_API_KEY", "gemini-3.8-flash"},
	"groq":   {"GROQ_API_KEY", "openai/gpt-oss-120b"},
	"echo":   {"", ""},
}

// DefaultKeyEnv returns the key environment variable a provider uses.
func DefaultKeyEnv(provider string) (string, bool) {
	d, ok := providerDefaults[strings.ToLower(provider)]
	return d.KeyEnv, ok
}

// defaultModel returns the default model for a provider, or "" if unknown.
func defaultModel(provider string) string {
	return providerDefaults[strings.ToLower(provider)].Model
}

// Defaults returns the configuration used when nothing is specified.
func Defaults() Config {
	return Config{
		Addr:      "127.0.0.1:8787",
		DataDir:   defaultDataDir(),
		Provider:  "gemini",
		Model:     defaultModel("gemini"),
		APIKeyEnv: "GEMINI_API_KEY",
		Timeout:   60 * time.Second,
		Source:    map[string]string{},
	}
}

// defaultDataDir picks a per-user location that works on both desktop and
// Termux, where $HOME exists but $XDG_DATA_HOME usually does not.
func defaultDataDir() string {
	if v := os.Getenv("AGENTFORGE_DATA_DIR"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ".agentforge"
	}
	return filepath.Join(home, ".agentforge")
}

// Load resolves configuration from defaults, then environment, then explicit
// overrides, in that order. Later sources win.
//
// Explicit values are passed as a map so that the CLI and tests can both use
// this without importing the flag package.
func Load(overrides map[string]string) (Config, error) {
	cfg := Defaults()

	// Environment first, so an override can still win below.
	envMap := map[string]string{
		"AGENTFORGE_ADDR":     "Addr",
		"AGENTFORGE_DATA_DIR": "DataDir",
		"AGENTFORGE_PROVIDER": "Provider",
		"AGENTFORGE_MODEL":    "Model",
		"AGENTFORGE_KEY_ENV":  "APIKeyEnv",
	}
	for envName, field := range envMap {
		if v := strings.TrimSpace(os.Getenv(envName)); v != "" {
			if err := cfg.setField(field, v, "env "+envName); err != nil {
				return cfg, err
			}
		}
	}

	// Choosing a provider must also adopt that provider's key variable and
	// default model, unless they were given explicitly. This runs after the
	// env pass so AGENTFORGE_PROVIDER=groq works, and before the override
	// pass so --model and --key-env still win.
	//
	// providerName is the flag value if set, otherwise whatever the env pass
	// resolved. Reading cfg.Provider alone would be wrong: a flag applied
	// later has not run yet.
	providerName := strings.ToLower(cfg.Provider)
	if p, ok := overrides["Provider"]; ok && p != "" {
		providerName = strings.ToLower(p)
	}
	if def, ok := providerDefaults[providerName]; ok {
		_, keyFromFlag := overrides["APIKeyEnv"]
		if !keyFromFlag && cfg.Source["APIKeyEnv"] == "" && def.KeyEnv != "" {
			cfg.APIKeyEnv = def.KeyEnv
			cfg.Source["APIKeyEnv"] = "provider default"
		}
		_, modelFromFlag := overrides["Model"]
		if !modelFromFlag && cfg.Source["Model"] == "" {
			cfg.Model = def.Model
			if def.Model != "" {
				cfg.Source["Model"] = "provider default"
			}
		}
	}
	if v := strings.TrimSpace(os.Getenv("AGENTFORGE_TIMEOUT")); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return cfg, fmt.Errorf("AGENTFORGE_TIMEOUT: %w", err)
		}
		cfg.Timeout = d
		cfg.Source["Timeout"] = "env AGENTFORGE_TIMEOUT"
	}

	// Explicit overrides last — these come from CLI flags.
	for field, v := range overrides {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if field == "Timeout" {
			d, err := time.ParseDuration(v)
			if err != nil {
				return cfg, fmt.Errorf("timeout: %w", err)
			}
			cfg.Timeout = d
			cfg.Source["Timeout"] = "flag --timeout"
			continue
		}
		if err := cfg.setField(field, v, "flag"); err != nil {
			return cfg, err
		}
	}

	return cfg, cfg.Validate()
}

// setField assigns one field by name. Kept separate from Load so the
// "where did this come from" bookkeeping happens in exactly one place.
func (c *Config) setField(field, value, source string) error {
	switch field {
	case "Addr":
		c.Addr = value
	case "DataDir":
		c.DataDir = value
	case "Provider":
		c.Provider = strings.ToLower(value)
	case "Model":
		c.Model = value
	case "APIKeyEnv":
		c.APIKeyEnv = value
	case "Timeout":
		d, err := time.ParseDuration(value)
		if err != nil {
			return fmt.Errorf("timeout: %w", err)
		}
		c.Timeout = d
	default:
		return fmt.Errorf("unknown config field %q", field)
	}
	c.Source[field] = source
	return nil
}

// validProviders are the backends the MVP actually implements.
var validProviders = map[string]bool{
	"gemini": true,
	"groq":   true,
	"echo":   true,
}

// Validate rejects configurations that would fail confusingly later.
func (c Config) Validate() error {
	if c.Addr == "" {
		return errors.New("addr is empty")
	}
	if c.DataDir == "" {
		return errors.New("data dir is empty")
	}
	if !validProviders[c.Provider] {
		return fmt.Errorf("unknown provider %q (want one of: echo, gemini, groq)",
			c.Provider)
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive, got %s", c.Timeout)
	}
	// "echo" is the offline provider, so it needs no key and no model.
	if c.Provider != "echo" {
		if c.Model == "" {
			return fmt.Errorf("provider %q needs a model id", c.Provider)
		}
		if c.APIKeyEnv == "" {
			return fmt.Errorf("provider %q needs an api key env var name", c.Provider)
		}
	}
	return nil
}

// APIKey returns the provider key from the environment, or "" if unset.
//
// It is read at call time rather than at load time so that a key added after
// start-up works without a restart — a real annoyance on a phone where you
// edit .env and re-run the same command.
func (c Config) APIKey() string {
	if c.APIKeyEnv == "" {
		return ""
	}
	return strings.TrimSpace(os.Getenv(c.APIKeyEnv))
}

// DatabasePath is where conversation state lives: one file, deletable.
func (c Config) DatabasePath() string {
	return filepath.Join(c.DataDir, "agentforge.db")
}

// Redacted returns a copy safe to print. The key is never in the struct in
// the first place, but this keeps that guarantee explicit.
func (c Config) Redacted() map[string]string {
	return map[string]string{
		"addr":        c.Addr,
		"data_dir":    c.DataDir,
		"provider":    c.Provider,
		"model":       c.Model,
		"api_key_env": c.APIKeyEnv,
		"timeout":     c.Timeout.String(),
		"key_set":     strconv.FormatBool(c.APIKey() != ""),
	}
}
