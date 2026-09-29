# AgentForge

[![test](https://github.com/fsix7115-arch/AgentForge/actions/workflows/test.yml/badge.svg)](https://github.com/fsix7115-arch/AgentForge/actions/workflows/test.yml)
[![release](https://github.com/fsix7115-arch/AgentForge/actions/workflows/release.yml/badge.svg)](https://github.com/fsix7115-arch/AgentForge/actions/workflows/release.yml)
[![Go](https://img.shields.io/badge/Go-1.24-00ADD8?logo=go&logoColor=white)](go.mod)
[![tests](https://img.shields.io/badge/tests-51-brightgreen)](.github/workflows/test.yml)
[![License](https://img.shields.io/badge/license-see%20LICENSE-blue)](LICENSE)

**A self-hosted AI agent server. One binary, runs on a phone.**

```bash
go build -o agentforge ./cmd/agentforge     # 10 MB, no dependencies
./agentforge doctor                         # can this device run it?
./agentforge chat "hello"                   # talk to it
./agentforge serve                          # local HTTP server
```

> **Status: MVP verified on a real phone.** A single binary with 51 tests, and
> confirmed running on an actual Android device — Android 15, arm64, inside
> Termux, built from source in about 6 minutes. No browser UI, no plugin
> system, no messaging integration yet. See [Verified on a real
> phone](#verified-on-a-real-phone).

---

## Relationship to other projects

This is **not** the same thing as
[`agentforge-compliance-hub`](https://github.com/fsix7115-arch/agentforge-compliance-hub),
despite the similar name. They are separate projects that happen to share a
brand:

| | This repo | agentforge-compliance-hub |
|---|---|---|
| What it is | A self-hosted AI agent **runtime** — one binary, runs on a phone | A **governance** platform for fleets of agents — audit, policy, cost, approvals |
| Language | Go | TypeScript / Next.js |
| What it answers | "Where can I run my agent?" | "What did my agents do, and was it allowed?" |
| State | MVP verified on Android 15 / arm64 | MVP, SQLite local, Docker for Postgres |

They can be used together — this is the runtime, that is the control plane — but
neither depends on the other, and either is useful alone.

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

This is the target, and it is the part that is not yet proven.

```bash
curl -fsSL https://raw.githubusercontent.com/fsix7115-arch/AgentForge/main/scripts/termux-check.sh | bash
```

That one script checks the device, installs Go if missing, clones, builds, and
verifies the binary end to end, then prints a verdict. Read
[`docs/TERMUX.md`](docs/TERMUX.md) first — it has the manual path, the
storage requirements, and troubleshooting.

Termux must come from **F-Droid**, not the Play Store: the Play Store build
has no `pkg` and therefore no Go.

The build for `android/arm64` succeeds, which is necessary but not sufficient.
**It has never been run on a real device.** That is the project's largest gap
and the reason this page asks for evidence rather than testimonials.

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
config   10 tests   resolution order, provider defaults, validation
agent     9 tests   offline provider, key handling, history trimming
store     9 tests   atomicity, turn limit, corruption, concurrency
server   12 tests   endpoints, validation, error paths, headers
doctor   11 tests   offline guarantee, key redaction, port conflict
```

```bash
go test ./...              # 51 tests
go test -race ./...        # concurrency
go vet ./...
```

Verified building for linux/amd64, linux/arm64, linux/386, android/arm64,
darwin/arm64, windows/amd64, windows/arm64. `android/amd64` is not a target:
it needs cgo, and the phone this project targets is arm64.

Live-tested against real Gemini and Groq endpoints, not only mocks.

## Verified on a real phone

**This is the section the whole project exists for.** Until a real phone ran
it, the central claim was unproven. Here is the output from one that did.

Device: Android 15 (SDK 35), aarch64, Termux, Go 1.27.1.
Run: `scripts/termux-check.sh`. Result: **13 passed, 0 failed, 1 warning.**

```
== 1. the device ==
  ok   CPU is aarch64 (64-bit) — the target architecture
  ok   running inside Termux (F-Droid or Termux:API build)
  ok   Android 15 (SDK 35)

== 2. the build toolchain ==
  ok   git present
  ok   go present (go1.27.1)
  ok   61278 MB free

== 3. build AgentForge from source ==
  ok   binary built
  ok   binary size 11M

== 4. does the binary actually run on this phone? ==
agentforge 0.1.0-dev
  ok   the binary executes on this device

== 5. agentforge doctor ==
[ok  ] platform      android/arm64
[ok  ] data-dir      .../agentforge-check/state — writable, 61278 MB free
[ok  ] session       .../agentforge.db — 2 turns, 468 bytes
[ok  ] port          127.0.0.1:8787 is free
All checks passed.

== 6. a real conversation turn ==
  ok   echo[3]: reply with the word: phone works

== 7. the HTTP server ==
  ok   server started and /health answered
       {"offline":true,"ok":true,"provider":"echo","turns":4}

  THIS PHONE CAN RUN AGENTFORGE.
```

What this does and does not prove:

- **Proves** the build works on a real ARM Android device, the binary executes,
  `doctor` passes, history persists to a file the user can read, and the HTTP
  server binds and answers.
- **Does not prove** anything about a real model on a phone. The test used the
  `echo` provider, which makes no network call. Running Gemini or Groq from a
  phone is the obvious next check and has not been done.

## Honest status

**Works:** the binary builds, all tests pass, `doctor`, `chat`, and `serve` run,
real Gemini and Groq replies verified on a laptop, cross-compiles to seven
targets, and **verified end to end on a real Android phone.**

**Not done:**

- **A real model has not been run from the phone.** The device test used the
  `echo` provider, which makes no network call. Gemini or Groq from a phone is
  the next check, and it is a different claim from "the binary runs".
- Reboot survival is not implemented. `termux-boot` is documented in
  [`docs/TERMUX.md`](docs/TERMUX.md) but the project does not set it up.
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
| 3. Architecture | done — [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) |
| 4. Learning roadmap | done — [`docs/LEARNING_ROADMAP.md`](docs/LEARNING_ROADMAP.md) |
| 5. Development roadmap | done — [`docs/DEVELOPMENT_ROADMAP.md`](docs/DEVELOPMENT_ROADMAP.md) |
| 6. Implementation | **partial** — CLI, server, tests, three providers |
| 7. Quality check | done — [`docs/QUALITY.md`](docs/QUALITY.md) |
| 8. Open-source release | done — [`CONTRIBUTING.md`](CONTRIBUTING.md), issue templates |

## License

MIT.

## Documentation

- [`docs/TERMUX.md`](docs/TERMUX.md) — the phone install, storage requirements,
  reboot survival, and troubleshooting. **Read this first if you have a phone.**
- [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) — how the code is structured,
  why each decision was made, and what the tradeoffs are
- [`docs/QUALITY.md`](docs/QUALITY.md) — how to run tests, what each test
  covers, common bugs, and security/performance notes
- [`docs/LEARNING_ROADMAP.md`](docs/LEARNING_ROADMAP.md) — beginner path
  through Git, Go, APIs, databases, AI APIs, and more
- [`docs/DEVELOPMENT_ROADMAP.md`](docs/DEVELOPMENT_ROADMAP.md) — the
  week-by-week build plan, with goals and success criteria for each phase
- [`docs/RESEARCH.md`](docs/RESEARCH.md) — ten projects compared, with live
  GitHub data and the corrections Phase 2 forced
- [`docs/VALIDATION.md`](docs/VALIDATION.md) — why the positioning narrowed to
  one idea, and what the MVP deliberately excludes
- [`docs/AGENTFORGE_PROMPT.md`](docs/AGENTFORGE_PROMPT.md) — the build prompt,
  with two added rules and the reasoning for each
- [`CONTRIBUTING.md`](CONTRIBUTING.md) — dev setup, code style, how to submit
  a PR, and good first issues
- [`scripts/termux-check.sh`](scripts/termux-check.sh) — one command that
  proves whether a given phone can run this

## How to help

The most useful thing anyone can do is run
[`scripts/termux-check.sh`](scripts/termux-check.sh) on a real Android phone
and open an issue with the output. Success and specific failure are both
valuable; silence leaves the central claim unproven.
