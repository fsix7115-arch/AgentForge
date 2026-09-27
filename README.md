# AgentForge

**A self-hosted AI agent server. One binary, runs on a phone.**

```bash
go build -o agentforge ./cmd/agentforge     # 10 MB, no dependencies
./agentforge doctor                         # can this device run it?
./agentforge chat "hello"                   # talk to it
./agentforge serve                          # local HTTP server
```

> **Status: working MVP (Phase 6 partial).** There is a real binary with real
> tests. There is no browser UI, no plugin system, and no messaging
> integration. The phone install — the whole positioning — is not tested on a
> real device yet. See [Honest status](#honest-status).

---

## Why phone-first

Phase 1 reviewed ten projects. Phase 2 tested the three candidate positions
against the market. Two were already occupied:

| Position | Verdict |
|---|---|
| Easiest-to-install agent server | **Taken** — [Gormes](https://gormes.ai/) ships a 55.6 MB MIT binary for Linux/macOS/Windows/Android; Ollama set the bar earlier |
| One agent, many surfaces, one memory | **Taken** — that is [Hermes](https://github.com/NousResearch/hermes-agent), 249k stars, actively developed |
| **Runs on a phone, first-class** | **The one left** |

Nobody in the ten-plus projects reviewed ships Termux as a first-class target.
The Termux story everywhere else is a third-party blog post.

Full reasoning, including the projects Phase 1 missed and the corrections it
needed: [`docs/RESEARCH.md`](docs/RESEARCH.md), [`docs/VALIDATION.md`](docs/VALIDATION.md).

## What it does

```bash
$ agentforge doctor
[ok  ] version       0.1.0-dev
[ok  ] platform      linux/arm64
[ok  ] config        addr=127.0.0.1:8787 provider=echo
[ok  ] data-dir      ~/.agentforge — writable, 4.2 GB free
[ok  ] session       not created yet (normal)
[ok  ] provider-key  echo provider needs no key
[ok  ] port          127.0.0.1:8787 is free

All checks passed. Run `agentforge serve` to start.
```

`doctor` makes **no network call and spends no token** unless you pass `--net`.
That is a hard requirement from Phase 2: a phone on a bad connection must still
get a useful answer.

```bash
$ agentforge chat "what is 2+2"
4

$ curl -s localhost:8787/chat -d '{"message":"hi"}'
{"reply":"hello","provider":"gemini","model":"gemini-3.8-flash","turns":2,"elapsed":"840ms"}
```

## Install

Requires Go 1.22+ to build. There is no release binary yet.

```bash
git clone https://github.com/fsix7115-arch/AgentForge
cd AgentForge
go test ./...
go build -o agentforge ./cmd/agentforge
./agentforge doctor
```

### On a phone (Termux)

This is the target, and it is the part that is not yet proven. The expected
path:

```bash
pkg install golang git
git clone https://github.com/fsix7115-arch/AgentForge
cd AgentForge
go build -o agentforge ./cmd/agentforge
./agentforge doctor
```

The build for `android/arm64` succeeds, which is necessary but not sufficient.
It has never been run on a device.

## Configuration

Flags beat environment variables beat defaults.

| Flag | Env | Default |
|---|---|---|
| `--addr` | `AGENTFORGE_ADDR` | `127.0.0.1:8787` |
| `--provider` | `AGENTFORGE_PROVIDER` | `gemini` |
| `--model` | `AGENTFORGE_MODEL` | per provider |
| `--key-env` | `AGENTFORGE_KEY_ENV` | per provider |
| `--data-dir` | `AGENTFORGE_DATA_DIR` | `~/.agentforge` |
| `--timeout` | `AGENTFORGE_TIMEOUT` | `60s` |
| `--turn-limit` | — | `40` |

Providers: `echo` (offline, no key), `gemini`, `groq`.

```bash
export GEMINI_API_KEY=...
agentforge chat "hello"

agentforge chat --provider groq "hello"     # key var switches automatically
```

## Architecture

```
cmd/agentforge        CLI entry: flag parsing, subcommand dispatch
├── internal/config   resolution order, validation, provider defaults
├── internal/agent    Agent interface + echo, gemini, groq backends
├── internal/store    conversation history, atomic JSON writes
├── internal/server   HTTP: /health, /chat, /clear
├── internal/doctor   offline diagnostics, exit 1 on failure
└── internal/version  build identity
```

Three decisions worth explaining:

**Loopback by default.** An agent that can run code must not be reachable from
the network by accident. Binding `0.0.0.0` is a flag, not a default.

**JSON history, not SQLite.** Gormes uses SQLite, which is the right long-term
answer. The MVP uses a single JSON file because on a phone you can `cat` it,
`rm` it, and recover it by hand if it corrupts. Writes are atomic — temp file
then rename — because Android kills processes without warning.

**Three endpoints, no auth, no CORS.** One user, one device, loopback. Adding
auth before there is a second user is work nobody needs yet.

## Testing

```
config   11 tests   resolution order, provider defaults, validation
agent     9 tests   offline provider, key handling, history trimming
store    10 tests   atomicity, turn limit, corruption, concurrency
server   12 tests   endpoints, validation, error paths, headers
doctor   11 tests   offline guarantee, key redaction, port conflict
```

```bash
go test ./...              # 53 tests
go test -race ./...        # concurrency
go vet ./...
```

Verified building for linux/amd64, linux/arm64, linux/386, android/arm64,
android/amd64, darwin/arm64, darwin/amd64, windows/amd64, windows/arm64.

Live-tested against real Gemini and Groq endpoints, not only mocks.

## Honest status

**Works:** the binary builds, all tests pass, `doctor`, `chat`, and `serve` run,
real Gemini and Groq replies verified, cross-compiles to nine targets.

**Not done:**

- **Not tested on a real phone.** The positioning is phone-first and the device
  test has not happened. This is the single largest gap.
- No messaging interface. The original vision listed terminal, browser, API,
  and future mobile apps; only the terminal and API exist.
- No browser UI, deliberately. Open WebUI and LibreChat saturate that space.
- No tools, no plugins, no auth, no streaming.
- No release binaries, so the install above requires a Go toolchain — exactly
  the friction the project exists to remove.

**Bugs found and fixed during this build,** each now covered by a test:

- `--provider groq` still read `GEMINI_API_KEY`, producing a 401 that looked
  like an invalid key rather than a wrong variable
- `--key-env` was in the help text but never defined as a flag
- a message of only whitespace was accepted and forwarded to the provider
- `freeBytes` was declared in two files at once, breaking non-Linux builds
- the non-Linux fallback file had a build tag that also matched Windows,
  so Windows builds failed entirely

## Roadmap

| Phase | Status |
|---|---|
| 1. Market research | done — [`docs/RESEARCH.md`](docs/RESEARCH.md) |
| 2. Validation and MVP | done — [`docs/VALIDATION.md`](docs/VALIDATION.md) |
| 3. Architecture | implicit in the code; written up pending |
| 4. Learning roadmap | not started |
| 5. Development roadmap | not started |
| 6. Implementation | **partial** — CLI, server, tests, three providers |
| 7. Quality check | partial — tests written, not audited |
| 8. Open-source release | README and LICENSE only |

## License

MIT.

## Documentation

- [`docs/RESEARCH.md`](docs/RESEARCH.md) — ten projects compared, with live
  GitHub data and the corrections Phase 2 forced
- [`docs/VALIDATION.md`](docs/VALIDATION.md) — why the positioning narrowed to
  one idea, and what the MVP deliberately excludes
- [`docs/AGENTFORGE_PROMPT.md`](docs/AGENTFORGE_PROMPT.md) — the build prompt,
  with two added rules and the reasoning for each
