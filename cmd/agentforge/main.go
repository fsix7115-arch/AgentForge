// Command agentforge is a self-hosted AI agent server.
//
// The MVP surface is four commands:
//
//	agentforge version   — what am I running
//	agentforge doctor    — can I run here (offline, spends no tokens)
//	agentforge chat      — one message, printed, then exits
//	agentforge serve     — the local HTTP server
//	agentforge clear     — delete local conversation history
//
// Everything else is deliberately absent. See docs/VALIDATION.md.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/fsix7115-arch/AgentForge/internal/agent"
	"github.com/fsix7115-arch/AgentForge/internal/config"
	"github.com/fsix7115-arch/AgentForge/internal/doctor"
	"github.com/fsix7115-arch/AgentForge/internal/server"
	"github.com/fsix7115-arch/AgentForge/internal/store"
	"github.com/fsix7115-arch/AgentForge/internal/version"
)

func main() {
	// Signal handling first, so Ctrl-C during a long model call still shuts
	// down gracefully and flushes state.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "agentforge: "+err.Error())
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		usage()
		return nil
	}

	cmd := args[0]
	fs := flag.NewFlagSet("agentforge "+cmd, flag.ContinueOnError)
	fs.Usage = func() { usage() }

	// Shared flags, accepted by every subcommand so that
	// `agentforge chat --provider groq` works without a global flag block.
	var (
		addr     = fs.String("addr", "", "listen address for serve (default 127.0.0.1:8787)")
		dataDir  = fs.String("data-dir", "", "where to store conversation history")
		provider = fs.String("provider", "", "model backend: echo, gemini, groq")
		model    = fs.String("model", "", "model id for the chosen provider")
		keyEnv   = fs.String("key-env", "", "env var name holding the provider key")
		timeout  = fs.String("timeout", "", "per-request timeout, e.g. 60s")
		turns    = fs.Int("turn-limit", 0, "how many turns of history to keep")
		verbose  = fs.Bool("verbose", false, "show every check in doctor, not just failures")
		netProbe = fs.Bool("net", false, "in doctor, also probe provider reachability")
		asJSON   = fs.Bool("json", false, "machine-readable output where supported")
	)

	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	switch cmd {
	case "version", "--version", "-v":
		fmt.Println("agentforge " + version.String())
		return nil

	case "help", "--help", "-h":
		usage()
		return nil
	}

	// Build overrides from only the flags the user actually set, so an unset
	// flag does not blank out the environment.
	overrides := map[string]string{}
	set := map[string]*string{
		"Addr": addr, "DataDir": dataDir, "Provider": provider,
		"Model": model, "Timeout": timeout,
	}
	for field, f := range set {
		if *f != "" {
			overrides[field] = *f
		}
	}
	if *keyEnv != "" {
		overrides["APIKeyEnv"] = *keyEnv
	} else if *provider != "" && *provider != config.Defaults().Provider {
		// Choosing a different provider must also switch to that provider's
		// default key variable. Without this, `--provider groq` silently
		// keeps reading GEMINI_API_KEY and fails with a confusing 401.
		if def, ok := config.DefaultKeyEnv(strings.ToLower(*provider)); ok {
			overrides["APIKeyEnv"] = def
		}
	}

	cfg, err := config.Load(overrides)
	if err != nil {
		return err
	}

	// log/slog with a level filter, defaulting to info. On a phone, debug
	// output is noise.
	logLevel := slog.LevelInfo
	if *verbose {
		logLevel = slog.LevelDebug
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel}))

	st := store.New(cfg.DatabasePath(), *turns)

	switch cmd {
	case "doctor":
		return runDoctor(ctx, cfg, st, *netProbe, *verbose, *asJSON)

	case "chat":
		return runChat(ctx, cfg, st, log, fs.Args())

	case "clear":
		if err := st.Clear(); err != nil {
			return err
		}
		fmt.Println("conversation history cleared")
		return nil

	case "serve":
		return runServe(ctx, cfg, st, log)

	default:
		return fmt.Errorf("unknown command %q (try: doctor, chat, serve, clear, version)", cmd)
	}
}

func runDoctor(ctx context.Context, cfg config.Config, st *store.Store,
	probeNet, verbose, asJSON bool) error {

	rep := doctor.Run(ctx, cfg, st, probeNet)

	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(map[string]any{"ok": rep.OK, "checks": rep.Checks}); err != nil {
			return err
		}
	} else {
		doctor.Render(os.Stdout, rep, verbose)
	}

	// Non-zero exit on failure so a setup script or CI can gate on it.
	// This is the same contract the devcheck tool uses.
	if !rep.OK {
		os.Exit(1)
	}
	return nil
}

func runChat(ctx context.Context, cfg config.Config, st *store.Store,
	log *slog.Logger, args []string) error {

	ag, err := agent.New(cfg)
	if err != nil {
		return err
	}

	// If a message was passed, answer it and exit. Otherwise read stdin, so
	// `echo "hi" | agentforge chat` and an interactive session both work.
	if len(args) > 0 {
		msg := strings.Join(args, " ")
		return oneShot(ctx, ag, st, log, msg)
	}

	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	fmt.Printf("agentforge chat — %s/%s. Ctrl-D or 'exit' to quit.\n", cfg.Provider, cfg.Model)

	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if line == "exit" || line == "quit" {
			return nil
		}
		if err := oneShot(ctx, ag, st, log, line); err != nil {
			// A failed turn should not kill an interactive session.
			fmt.Fprintln(os.Stderr, "error:", err)
			continue
		}
	}
	return sc.Err()
}

func oneShot(ctx context.Context, ag agent.Agent, st *store.Store,
	log *slog.Logger, msg string) error {

	sess, err := st.Load()
	if err != nil {
		// No session yet is the normal first-turn case.
		sess = store.Session{}
	}

	reply, err := ag.Reply(ctx, sess.Turns, msg)
	if err != nil {
		return err
	}
	if _, err := st.Append("user", msg); err != nil {
		log.Warn("could not persist turn", "err", err)
	}
	if _, err := st.Append("assistant", reply); err != nil {
		log.Warn("could not persist turn", "err", err)
	}
	fmt.Println(reply)
	return nil
}

func runServe(ctx context.Context, cfg config.Config, st *store.Store, log *slog.Logger) error {
	ag, err := agent.New(cfg)
	if err != nil {
		return fmt.Errorf("%w\n\nrun `agentforge doctor` for details", err)
	}
	srv := server.New(cfg, ag, st, log)
	return server.Serve(ctx, srv, log)
}

func usage() {
	fmt.Print(`agentforge — a self-hosted AI agent server

Usage:
  agentforge <command> [flags]

Commands:
  serve       start the local HTTP server
  chat        talk to the agent; reads stdin, or takes a message argument
  doctor      check whether this machine can run agentforge (offline, no tokens)
  clear       delete local conversation history
  version     print build identity
  help        this text

Flags (all commands):
  --addr string        listen address              (default 127.0.0.1:8787)
  --provider string    echo | gemini | groq        (default gemini)
  --model string       model id
  --data-dir string    where to keep history
  --timeout duration   per-request timeout         (default 60s)
  --turn-limit int     turns of history to keep    (default 40)
  --verbose            more detail, including every doctor check
  --json               machine-readable output
  --net                in doctor, also probe the provider endpoint

Examples:
  agentforge doctor --provider echo
  agentforge chat --provider echo "hello"
  agentforge serve --addr 127.0.0.1:8787
  curl -s localhost:8787/chat -d '{"message":"hi"}'

Environment:
  GEMINI_API_KEY, GROQ_API_KEY   provider keys
  AGENTFORGE_ADDR, AGENTFORGE_PROVIDER, AGENTFORGE_MODEL,
  AGENTFORGE_DATA_DIR, AGENTFORGE_TIMEOUT, AGENTFORGE_KEY_ENV

Docs: https://github.com/fsix7115-arch/AgentForge
`)
}
