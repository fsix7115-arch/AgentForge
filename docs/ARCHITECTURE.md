# AgentForge Architecture

This document describes how AgentForge is structured, why it is structured that way, and what the tradeoffs are. It is written for someone reading the code for the first time.

---

## System overview

AgentForge is a single Go binary. It does three things:

1. Talks to an AI provider from the command line (`agentforge chat`)
2. Exposes that conversation over a local HTTP API (`agentforge serve`)
3. Diagnoses itself without a network call (`agentforge doctor`)

The primary target is an Android phone running Termux. Every design decision is checked against the question: "does this work on a phone with intermittent data, limited storage, and no init system?"

**Design philosophy:** no background complexity. No daemon manager, no database engine, no plugin runtime. If something can be a plain file, it is. If something can be a function call instead of a service, it is. The binary should be understandable by someone who has not read a line of it before.

---

## Repository layout

```
AgentForge/
├── cmd/
│   └── agentforge/
│       └── main.go          entry point: flag parsing, subcommand dispatch
├── internal/
│   ├── agent/
│   │   └── agent.go         Agent interface + echo, gemini, groq implementations
│   ├── config/
│   │   └── config.go        resolution order, validation, provider defaults
│   ├── doctor/
│   │   ├── doctor.go        offline diagnostics, exit 1 on failure
│   │   ├── statfs_unix.go   free-space query for Linux/macOS/Android
│   │   └── statfs_windows.go free-space query for Windows
│   ├── server/
│   │   └── server.go        HTTP handlers, graceful shutdown
│   ├── store/
│   │   └── store.go         conversation history, atomic JSON writes
│   └── version/
│       └── version.go       build identity string
├── docs/                    all documentation
├── scripts/
│   └── termux-check.sh      one-command phone verification
├── go.mod                   module declaration, no external dependencies
└── LICENSE                  MIT
```

Each `internal/` package has one job. None of them import each other in a cycle. The dependency graph is a DAG with `cmd/agentforge/main.go` at the top.

---

## Backend architecture

### `cmd/agentforge`

`main.go` is the entry point and the only place that knows about all the packages. It does:

1. Sets up signal handling (`SIGINT`, `SIGTERM`) so Ctrl-C shuts down gracefully
2. Parses the subcommand (`doctor`, `chat`, `serve`, `clear`, `version`)
3. Parses flags — all flags are shared across subcommands so `agentforge chat --provider groq` works naturally
4. Builds overrides only from flags the user actually set (an unset flag does not overwrite an environment variable)
5. Calls `config.Load`, `store.New`, then delegates to the subcommand

**Why one flag set for all commands?** On a phone you run `agentforge chat --provider gemini "hello"`. You do not want to remember whether `--provider` goes before or after the subcommand. One flag set makes that not matter.

**Why `flag.ContinueOnError`?** The binary prints usage and returns a non-zero exit, rather than calling `os.Exit` directly from inside the flag package. This makes it testable.

### `internal/config`

Resolves settings in this order: defaults → environment variables → CLI flags. Later sources win.

```
defaults
  └─ env AGENTFORGE_ADDR, AGENTFORGE_PROVIDER, ...
       └─ flag --addr, --provider, ...
```

The provider table is key:

```go
var providerDefaults = map[string]struct{ KeyEnv, Model string }{
    "gemini": {"GEMINI_API_KEY", "gemini-3.8-flash"},
    "groq":   {"GROQ_API_KEY",   "openai/gpt-oss-120b"},
    "echo":   {"", ""},
}
```

When you switch providers, the key variable switches too. This table is why `--provider groq` automatically reads `GROQ_API_KEY` instead of `GEMINI_API_KEY`. Without it, a real bug existed: the provider switched but the key variable did not, producing a 401 that looked like a bad key.

**Tradeoff — no config file:** The MVP has no `~/.agentforge/config.toml`. Everything is flags or environment variables. This is simpler to document and harder to corrupt. The downside is that you have to `export GEMINI_API_KEY=...` in `.bashrc` yourself, rather than having the binary manage a config file.

### `internal/version`

Single file, holds the version string. Separated so it can be replaced at build time:

```bash
go build -ldflags "-X github.com/fsix7115-arch/AgentForge/internal/version.Version=0.1.0" ./cmd/agentforge
```

---

## API architecture

The HTTP server exposes four routes:

```
GET  /         — list available endpoints (discovery)
GET  /health   — liveness check: version, provider, turn count
POST /chat     — {message: string} → {reply, provider, model, turns, elapsed}
POST /clear    — delete local conversation history
```

### Request flow for POST /chat

```
client
  │
  ▼
server.handleChat
  │  parse JSON body, trim whitespace, reject empty message
  │
  ▼
store.Load      ← reads ~/.agentforge/agentforge.db (or client-supplied history)
  │
  ▼
agent.Reply     ← sends request to provider with timeout
  │             ← gemini: POST generativelanguage.googleapis.com
  │             ← groq:   POST api.groq.com/openai/v1/chat/completions
  │             ← echo:   returns immediately, no network call
  ▼
store.Append    ← atomic write: temp file → rename
  │
  ▼
JSON response   {reply, provider, model, turns, elapsed}
```

**Why no streaming?** Streaming requires either SSE or chunked transfer encoding, both of which need different client handling. The MVP serves one user on one device; a 1-second wait is acceptable. Streaming is listed as not-done rather than cut quietly.

**Why no CORS headers?** The server binds loopback by default. A page on the local machine that tried a cross-origin request would still fail, because the server sends no `Access-Control-Allow-Origin` header intentionally — a malicious page cannot read responses even if the listen address changes.

**Why no auth?** One user, one device, loopback. Authentication before there is a second user is work nobody needs yet. If you expose `--addr 0.0.0.0:8787`, you accept the risk. That is documented, not hidden.

**Body size limit:** `POST /chat` caps the request body at 1 MB. A phone has limited memory, and an accidental file upload through `curl` should not fill it.

---

## Memory and storage architecture

### The session file

All conversation history lives in one JSON file:

```
~/.agentforge/agentforge.db
```

The `.db` extension is a historical artifact from when SQLite was considered. The file is JSON:

```json
{
  "id": "default",
  "created": "2026-01-01T12:00:00Z",
  "updated": "2026-01-01T12:05:00Z",
  "turn_limit": 40,
  "turns": [
    {"role": "user",      "content": "hello", "ts": "2026-01-01T12:00:00Z"},
    {"role": "assistant", "content": "hi",    "ts": "2026-01-01T12:00:01Z"}
  ]
}
```

**Why JSON and not SQLite?** Gormes (the closest competitor) uses SQLite, which is the right long-term answer. The MVP does not because:

- SQLite needs cgo or a pure-Go driver. Both add build complexity. `CGO_ENABLED=0` is required to cross-compile for `android/arm64` from a laptop, and cgo breaks that.
- A JSON file can be read with `cat`, deleted with `rm`, and manually edited if it corrupts. On a phone, that matters.
- The interface (`Store`) is the seam. Replacing JSON with SQLite later only requires a new implementation of `Load`, `Append`, `Clear`, and `Stats`.

**Tradeoff:** JSON is slower for large histories and less efficient. A 40-turn limit (configurable) keeps the file small. At 40 turns and ~200 bytes per turn, the file is about 8 KB.

### Atomic writes

```go
tmp := s.path + ".tmp"
os.WriteFile(tmp, b, 0o600)
os.Rename(tmp, s.path)
```

Android kills processes without warning. A write that is interrupted mid-file leaves a partial JSON document, which is unreadable. Write-to-temp-then-rename is atomic on the same filesystem: either the old file exists or the new one does, never a partial one.

### File permissions

The session file is created with `0o600` (owner read/write only). On a shared or rooted phone, the history is not readable by other apps. The data directory is created with `0o700` for the same reason.

### Turn limit and trimming

When the session exceeds `turn_limit` turns, the oldest turns are dropped:

```go
if len(sess.Turns) > sess.TurnLimit {
    sess.Turns = sess.Turns[len(sess.Turns)-sess.TurnLimit:]
}
```

Recent context is more useful than old context. A phone has limited storage. The limit defaults to 40 turns and is configurable with `--turn-limit`.

---

## Provider and agent architecture

### The Agent interface

```go
type Agent interface {
    Name()    string
    Offline() bool
    Reply(ctx context.Context, history []store.Turn, userMsg string) (string, error)
}
```

Three methods. Adding a fourth provider means implementing these three. The server and CLI never import a specific provider — they talk to the interface.

### echo

The offline provider. Returns immediately with no network call:

```go
func (a *echoAgent) Reply(ctx context.Context, history []store.Turn, userMsg string) (string, error) {
    n := len(history) + 1
    return fmt.Sprintf("echo[%d]: %s (offline provider, no model called)", n, userMsg), nil
}
```

Used for: CI, `doctor` checks, phone verification, development without an API key.

**Advantage:** Works anywhere, costs nothing, never fails from a network issue.
**Disadvantage:** Not an AI. It echoes.

### gemini

Calls the Google Generative Language API:

```
POST https://generativelanguage.googleapis.com/v1beta/models/{model}:generateContent
x-goog-api-key: {key}
```

History is serialized as a single text prompt with `User:` / `Assistant:` prefixes (not as a structured multi-turn message list). This is simpler and works with the current API shape; a future version should use the structured form.

**Advantage:** Free tier is generous. No special setup beyond an API key from Google AI Studio.
**Disadvantage:** Sends the whole history as one text block. Long conversations are less efficient than native multi-turn.

### groq

Calls the Groq chat completions API (OpenAI-compatible):

```
POST https://api.groq.com/openai/v1/chat/completions
Authorization: Bearer {key}
```

Groq uses a system prompt (`"You are AgentForge, a helpful assistant running on the user's own device."`) and sends history as a single user message. A browser-style `User-Agent` header is set because Cloudflare's bot detection on the Groq API rejects the default Go user-agent with error 1010.

**Advantage:** Very fast inference. Free tier available.
**Disadvantage:** Needs a Groq API key. The browser UA workaround is fragile and may need updating.

### Adding a new provider

1. Add a struct that implements `Agent`
2. Add its key variable and default model to `providerDefaults` in `config.go`
3. Add a case to the `switch` in `agent.New`
4. Add it to `validProviders` in `config.go`
5. Add at least the key-switching test in `config_test.go`

No other files need to change.

---

## Security architecture

**Loopback by default.** The server binds `127.0.0.1:8787`. An agent that can (in the future) execute code must not be reachable from the network by accident. Binding `0.0.0.0` requires an explicit flag.

**No authentication.** The MVP assumes one user on one device. Auth before a second user exists is scope creep. If you expose the server on `0.0.0.0`, you are responsible for what that means.

**Keys are never stored.** API keys are read from environment variables at call time, not at startup. The config struct holds the variable name (`GEMINI_API_KEY`), never the value. `doctor --verbose` reports the variable name and character count, never the value.

**Session file is private.** `0o600` permissions. Other apps cannot read conversation history without root.

**No CORS.** The server does not send `Access-Control-Allow-Origin`. A browser page cannot read responses from the local server even if the address changes.

**Body size cap.** Requests are capped at 1 MB. This is not a security boundary (loopback only), but it prevents accidental large uploads from filling a phone's memory.

**What is not done:**

- No rate limiting
- No auth of any kind
- No HTTPS (loopback only; TLS on loopback is unusual and adds cert management complexity)
- No input sanitization beyond trimming whitespace (the provider API does the rest)

---

## Plugin architecture (future)

There is no plugin system. This is deliberate for the MVP.

When a plugin system is added, the natural extension point is the `Agent` interface. A plugin agent could:

- Wrap another agent and intercept calls
- Run a tool and inject the result into the next turn
- Route to different providers based on the message content

The `store.Turn` struct is the other extension point. A `metadata` field on turns would allow tool call records, citations, or tool results without breaking the JSON format.

The MVP does not implement any of this. The interface exists as the seam, not the feature.

---

## Data flow diagram

```
CLI input / HTTP request
         │
         ▼
   ┌─────────────┐
   │  main.go    │  flag parsing, signal handling, subcommand dispatch
   └──────┬──────┘
          │
    ┌─────▼──────┐
    │  config    │  defaults → env → flags
    └─────┬──────┘
          │
    ┌─────▼──────┐     ┌──────────┐
    │   store    │◄────│ .db file │  atomic read/write
    └─────┬──────┘     └──────────┘
          │
    ┌─────▼──────┐
    │   agent    │  Agent interface
    └─────┬──────┘
          │
   ┌──────┼──────┐
   ▼      ▼      ▼
 echo  gemini  groq        (one is selected at startup)
               │
               ▼
         provider API
         (network)
```

---

## Why Go

- **Single binary, no runtime.** `go build` produces one file. Copy it to a phone. Run it. No JVM, no Python interpreter, no shared libraries beyond libc (and with `CGO_ENABLED=0`, not even that).
- **Cross-compilation is first class.** `GOOS=android GOARCH=arm64 go build` works from any machine. The release workflow builds seven targets from one CI job.
- **Standard library is enough.** The HTTP server, JSON encoding, file I/O, signal handling, structured logging, and testing framework all come with Go. The `go.mod` has no external dependencies.
- **Fast build on constrained hardware.** A cold build on a phone takes 3-10 minutes. A Python or Node project with hundreds of npm/pip packages would take longer and leave more behind.
- **`go test` is a test runner.** No test framework to install, no config file, just `go test ./...`.

**Tradeoffs:**

- Go's garbage collector adds ~10 MB binary overhead. A C program would be smaller. That is acceptable.
- Go is more verbose than Python for scripting. It is a better fit here because this is a server, not a script.
- The standard library HTTP server does not support HTTP/2 push or WebSockets without extra packages. Neither is needed for the MVP.
