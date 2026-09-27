package doctor

import (
	"context"
	"net"
	"strings"
	"testing"

	"github.com/fsix7115-arch/AgentForge/internal/config"
	"github.com/fsix7115-arch/AgentForge/internal/store"
)

func testSetup(t *testing.T) (config.Config, *store.Store) {
	t.Helper()
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("GROQ_API_KEY", "")

	cfg := config.Defaults()
	cfg.Provider = "echo"
	cfg.Addr = "127.0.0.1:0" // never in use, so the port check passes
	cfg.DataDir = t.TempDir()

	st := store.New(cfg.DatabasePath(), 10)
	return cfg, st
}

func TestEchoProviderPassesOffline(t *testing.T) {
	cfg, st := testSetup(t)

	// probeNetwork false: the whole point of doctor is that it costs nothing.
	rep := Run(context.Background(), cfg, st, false)
	if !rep.OK {
		t.Fatalf("report not OK: %+v", rep.Checks)
	}

	want := []string{"version", "platform", "config", "data-dir", "session", "provider-key", "port"}
	have := map[string]bool{}
	for _, c := range rep.Checks {
		have[c.Name] = true
	}
	for _, w := range want {
		if !have[w] {
			t.Errorf("missing check %q", w)
		}
	}
}

// The default run must not contain any network check at all. A user on a
// phone with no data should get a useful report, not a timeout.
func TestNetworkCheckIsOmittedByDefault(t *testing.T) {
	cfg, st := testSetup(t)
	rep := Run(context.Background(), cfg, st, false)
	for _, c := range rep.Checks {
		if c.Name == "network" {
			t.Error("a network check ran during the offline doctor")
		}
	}
}

func TestMissingKeyIsFatalForRealProviders(t *testing.T) {
	cfg, st := testSetup(t)
	cfg.Provider = "gemini"
	cfg.APIKeyEnv = "GEMINI_API_KEY"
	cfg.Model = "gemini-3.8-flash"

	rep := Run(context.Background(), cfg, st, false)
	if rep.OK {
		t.Fatal("report OK despite a missing API key")
	}

	var found bool
	for _, c := range rep.Checks {
		if c.Name == "provider-key" {
			found = true
			if c.OK {
				t.Error("provider-key check passed with no key set")
			}
			if !c.Fatal {
				t.Error("a missing key must be fatal, not a warning")
			}
			if !strings.Contains(c.Detail, "GEMINI_API_KEY") {
				t.Errorf("detail = %q, should name the variable to set", c.Detail)
			}
		}
	}
	if !found {
		t.Error("no provider-key check was run")
	}
}

// The key value must never appear in any output.
func TestKeyValueIsNeverPrinted(t *testing.T) {
	cfg, st := testSetup(t)
	cfg.Provider = "gemini"
	cfg.APIKeyEnv = "GEMINI_API_KEY"
	cfg.Model = "gemini-3.8-flash"
	t.Setenv("GEMINI_API_KEY", "super-secret-key-value-12345")

	rep := Run(context.Background(), cfg, st, false)

	var b strings.Builder
	Render(&b, rep, true)

	if strings.Contains(b.String(), "super-secret-key-value-12345") {
		t.Fatal("the API key value was printed in doctor output")
	}
	if !strings.Contains(b.String(), "GEMINI_API_KEY") {
		t.Error("doctor should still name the variable, so the user knows what to set")
	}
}

func TestKeyLengthIsReportedButNotValue(t *testing.T) {
	cfg, st := testSetup(t)
	cfg.Provider = "gemini"
	cfg.APIKeyEnv = "GEMINI_API_KEY"
	cfg.Model = "gemini-3.8-flash"
	t.Setenv("GEMINI_API_KEY", "abcd")

	rep := Run(context.Background(), cfg, st, false)
	for _, c := range rep.Checks {
		if c.Name == "provider-key" {
			if !c.OK {
				t.Error("a set key should pass the check")
			}
			if !strings.Contains(c.Detail, "hidden") {
				t.Errorf("detail = %q, should state the value is hidden", c.Detail)
			}
		}
	}
}

func TestUnwritableDataDirIsFatal(t *testing.T) {
	cfg, st := testSetup(t)
	// A path under a file, not a directory, cannot be created.
	cfg.DataDir = "/etc/passwd/nope"
	st = store.New(cfg.DatabasePath(), 10)

	rep := Run(context.Background(), cfg, st, false)
	if rep.OK {
		t.Fatal("report OK despite an unusable data directory")
	}
	for _, c := range rep.Checks {
		if c.Name == "data-dir" {
			if c.OK {
				t.Error("data-dir check passed on an unwritable path")
			}
			if !c.Fatal {
				t.Error("an unwritable data dir must be fatal")
			}
		}
	}
}

func TestPortAlreadyInUseIsDetected(t *testing.T) {
	cfg, st := testSetup(t)

	// Bind a real port first, then point the config at it. Port 0 cannot be
	// used here: dialing :0 asks the OS for a *different* ephemeral port, so
	// the check would report "free" even though our own listener exists.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("could not bind a test listener: %v", err)
	}
	defer ln.Close()

	cfg.Addr = ln.Addr().String()

	rep := Run(context.Background(), cfg, st, false)
	if rep.OK {
		t.Fatal("report OK while the port is occupied")
	}
	for _, c := range rep.Checks {
		if c.Name == "port" {
			if c.OK {
				t.Error("port check passed on an occupied port")
			}
			if !strings.Contains(c.Detail, "already in use") {
				t.Errorf("detail = %q, should explain the conflict", c.Detail)
			}
		}
	}
}

func TestFirstRunSessionIsNotAnError(t *testing.T) {
	cfg, st := testSetup(t)
	rep := Run(context.Background(), cfg, st, false)
	for _, c := range rep.Checks {
		if c.Name == "session" {
			if !c.OK {
				t.Error("a missing session on first run should be a pass")
			}
			if !strings.Contains(c.Detail, "not created yet") {
				t.Errorf("detail = %q, should say it is normal", c.Detail)
			}
		}
	}
}

func TestExistingSessionIsReported(t *testing.T) {
	cfg, st := testSetup(t)
	if _, err := st.Append("user", "hello"); err != nil {
		t.Fatalf("append: %v", err)
	}

	rep := Run(context.Background(), cfg, st, false)
	for _, c := range rep.Checks {
		if c.Name == "session" {
			if !strings.Contains(c.Detail, "2 turns") && !strings.Contains(c.Detail, "1 turns") {
				t.Errorf("detail = %q, should report the turn count", c.Detail)
			}
		}
	}
}

func TestRenderOnlyShowsFailuresUnlessVerbose(t *testing.T) {
	cfg, st := testSetup(t)
	rep := Run(context.Background(), cfg, st, false)

	var quiet strings.Builder
	Render(&quiet, rep, false)
	if strings.Contains(quiet.String(), "[ok") {
		t.Error("non-verbose output should hide passing checks")
	}

	var loud strings.Builder
	Render(&loud, rep, true)
	if !strings.Contains(loud.String(), "[ok") {
		t.Error("verbose output should show passing checks")
	}
}

func TestRenderMarksWarningsDistinctlyFromFailures(t *testing.T) {
	rep := Report{
		OK: true,
		Checks: []Check{
			{Name: "fatal-thing", OK: false, Detail: "broken", Fatal: true},
			{Name: "minor-thing", OK: false, Detail: "odd", Fatal: false},
			{Name: "fine", OK: true, Detail: "all good", Fatal: false},
		},
	}
	var b strings.Builder
	Render(&b, rep, true)
	out := b.String()

	if !strings.Contains(out, "[FAIL]") {
		t.Error("a fatal failure should render as FAIL")
	}
	if !strings.Contains(out, "[warn]") {
		t.Error("a non-fatal failure should render as warn, not FAIL")
	}
}
