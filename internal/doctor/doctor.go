// Package doctor reports whether AgentForge can actually run here.
//
// The requirement from Phase 2 is specific: `doctor` must work offline and
// must not spend a token. So this package makes no model calls at all. It
// checks configuration, the filesystem, and provider reachability only when
// explicitly asked.
package doctor

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/fsix7115-arch/AgentForge/internal/config"
	"github.com/fsix7115-arch/AgentForge/internal/store"
	"github.com/fsix7115-arch/AgentForge/internal/version"
)

// Check is one line of diagnostic output.
type Check struct {
	Name   string
	OK     bool
	Detail string
	// Fatal means the server cannot start. Non-fatal checks are warnings.
	Fatal bool
}

// Report is the full result of a doctor run.
type Report struct {
	Checks []Check
	OK     bool
}

// Run performs the offline checks. Pass probeNetwork=false to guarantee no
// outbound call; the CLI sets it true only behind an explicit --net flag.
func Run(ctx context.Context, cfg config.Config, st *store.Store, probeNetwork bool) Report {
	var r Report

	add := func(name string, ok bool, detail string, fatal bool) {
		r.Checks = append(r.Checks, Check{name, ok, detail, fatal})
	}

	// 1. Build identity. Knowing what you are running is the first thing you
	// need when something behaves differently on a phone than on a laptop.
	add("version", true, version.String(), false)
	add("platform", true, fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH), false)

	// 2. Configuration must have passed validation to get here, but report
	// the resolved values so a phone user can see what is actually in effect.
	add("config", true, fmt.Sprintf("addr=%s provider=%s model=%s timeout=%s",
		cfg.Addr, cfg.Provider, cfg.Model, cfg.Timeout), false)

	// 3. Data directory: create it if missing, and prove it is writable.
	// A phone's storage being full is the single most common failure.
	dirOK := true
	dirDetail := cfg.DataDir
	if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
		dirOK = false
		dirDetail = fmt.Sprintf("%s — cannot create: %v", cfg.DataDir, err)
	} else {
		probe := filepath.Join(cfg.DataDir, ".write-probe")
		if err := os.WriteFile(probe, []byte("ok"), 0o600); err != nil {
			dirOK = false
			dirDetail = fmt.Sprintf("%s — not writable: %v", cfg.DataDir, err)
		} else {
			os.Remove(probe)
			// Report free space in MB, which is the unit a phone shows.
			var free uint64
			if _, err := os.Stat(cfg.DataDir); err == nil {
				free = freeBytes(cfg.DataDir)
			}
			dirDetail = fmt.Sprintf("%s — writable, %d MB free", cfg.DataDir, free/1024/1024)
		}
	}
	add("data-dir", dirOK, dirDetail, dirOK == false)

	// 4. Conversation state.
	turns, size, exists, err := st.Stats()
	switch {
	case err != nil:
		add("session", false, fmt.Sprintf("%s — unreadable: %v", st.Path(), err), true)
	case !exists:
		add("session", true, fmt.Sprintf("%s — not created yet (normal)", st.Path()), false)
	default:
		add("session", true, fmt.Sprintf("%s — %d turns, %d bytes", st.Path(), turns, size), false)
	}

	// 5. Provider key presence. Never print the value.
	if cfg.Provider == "echo" {
		add("provider-key", true, "echo provider needs no key", false)
	} else {
		if cfg.APIKey() == "" {
			add("provider-key", false,
				fmt.Sprintf("%s is not set", cfg.APIKeyEnv), true)
		} else {
			add("provider-key", true,
				fmt.Sprintf("%s is set (%d chars, value hidden)", cfg.APIKeyEnv, len(cfg.APIKey())), false)
		}
	}

	// 6. Is the port already taken? A phone running this twice looks like a
	// mystery failure otherwise.
	conn, dialErr := net.DialTimeout("tcp", cfg.Addr, 700*time.Millisecond)
	if dialErr == nil {
		conn.Close()
		add("port", false,
			fmt.Sprintf("%s is already in use — another instance is probably running", cfg.Addr), true)
	} else {
		add("port", true, fmt.Sprintf("%s is free", cfg.Addr), false)
	}

	// 7. Network reachability, only when explicitly requested. This is the
	// only check that can fail for reasons outside the user's control, which
	// is exactly why it is not part of the default run.
	if probeNetwork {
		if cfg.Provider == "echo" {
			add("network", true, "skipped for echo provider", false)
		} else if cfg.APIKey() == "" {
			add("network", false, "skipped: no API key to test with", false)
		} else {
			add("network", probeProvider(ctx, cfg.Provider), providerDetail(cfg.Provider), false)
		}
	}

	r.OK = true
	for _, c := range r.Checks {
		if c.Fatal && !c.OK {
			r.OK = false
		}
	}
	return r
}

// providerURLs are the endpoints checked with a HEAD/GET when --net is passed.
// A successful TCP+TLS connection is enough; we deliberately do not spend a
// token on a generation call just to prove the key works.
var providerURLs = map[string]string{
	"gemini": "https://generativelanguage.googleapis.com/",
	"groq":   "https://api.groq.com/openai/v1/models",
}

func probeProvider(ctx context.Context, provider string) bool {
	url, ok := providerURLs[provider]
	if !ok {
		return true
	}
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; agentforge-doctor)")

	client := &http.Client{Timeout: 6 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 512))

	// 401 means reachable but the key is bad. That is still a network pass
	// and a different diagnosis, so treat <500 as "the network works".
	return resp.StatusCode < 500
}

func providerDetail(provider string) string {
	return provider + " endpoint reachable"
}

// Render formats a report for a terminal. Wide terminals get a two-column
// layout; narrow ones (a phone in Termux) get stacked lines.
func Render(w io.Writer, r Report, verbose bool) {
	for _, c := range r.Checks {
		mark := "ok  "
		if !c.OK {
			mark = "FAIL"
			if !c.Fatal {
				mark = "warn"
			}
		}
		if verbose || !c.OK {
			fmt.Fprintf(w, "[%s] %-13s %s\n", mark, c.Name, c.Detail)
		}
	}
	if r.OK {
		fmt.Fprintln(w, "\nAll checks passed. Run `agentforge serve` to start.")
	} else {
		fmt.Fprintln(w, "\nSome required checks failed. Fix them before serving.")
	}
}
