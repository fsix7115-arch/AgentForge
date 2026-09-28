# Contributing to AgentForge

Thanks for looking at this. The project is phone-first — every change should be checked against "does this still work on a phone in Termux?"

---

## Set up a dev environment

You need Go 1.22 or later. Check with `go version`.

```bash
git clone https://github.com/fsix7115-arch/AgentForge
cd AgentForge
go test ./...        # should print "ok" five times
go build -o agentforge ./cmd/agentforge
./agentforge doctor --provider echo
```

No external dependencies. `go.mod` has only the Go standard library.

If you want to test a real provider:

```bash
export GEMINI_API_KEY=your_key_here
./agentforge chat "hello"

export GROQ_API_KEY=your_key_here
./agentforge chat --provider groq "hello"
```

---

## Run the tests

```bash
go test ./...              # all 51 tests
go test -race ./...        # race detector — run this before any PR
go vet ./...               # static analysis
```

What each test file covers:

```
internal/config/config_test.go   resolution order, provider defaults, validation
internal/agent/agent_test.go     offline echo provider, key handling, history trimming
internal/store/store_test.go     atomicity, turn limit, corruption, concurrency
internal/server/server_test.go   endpoints, validation, error paths, headers
internal/doctor/doctor_test.go   offline guarantee, key redaction, port conflict
```

Tests must pass with `CGO_ENABLED=0` because that is the cross-compile requirement for Android. If you add a dependency, check that it does not need cgo.

---

## Code style

The existing code follows standard Go style enforced by `go vet` and `gofmt`. A few project-specific rules:

**Comments explain why, not what.** The code shows what. The comment should say why you made a specific choice, especially if the choice is non-obvious or if a simpler approach was rejected.

```go
// Bad: these are the atomic write steps
tmp := s.path + ".tmp"
os.WriteFile(tmp, b, 0o600)
os.Rename(tmp, s.path)

// Good: Android kills processes without warning. Write-to-temp-then-rename
// means a crash leaves either the old file or the new one, never a partial one.
tmp := s.path + ".tmp"
os.WriteFile(tmp, b, 0o600)
os.Rename(tmp, s.path)
```

**No external dependencies without a good reason.** The binary currently has zero third-party packages. Before adding one, check whether the standard library handles it. The value of a zero-dependency binary on a phone is real.

**Keep the phone in mind.** A phone has less memory, slower storage, and an OS that kills processes without warning. Code that works fine on a laptop can fail on a phone. Specifically:
- Keep the binary small. Avoid packages that pull in large transitive dependencies.
- Do not hold large amounts of data in memory.
- Prefer atomic writes (temp file → rename) over in-place writes.

**Format before committing.** Run `gofmt -w .` or configure your editor to do it on save.

---

## How to submit a PR

1. Fork the repo on GitHub
2. Create a branch from `main` — name it something descriptive (`fix-groq-key-switch`, `add-openai-provider`)
3. Make your changes
4. Run `go test -race ./...` and `go vet ./...` — both must pass
5. Run `go build -o /dev/null ./cmd/agentforge` — the binary must still build
6. Open a PR against `main` with a description of what changed and why
7. If the change is testable on a phone, say so in the description

PRs that break the test suite or fail to build will not be merged. PRs that add functionality but skip tests will be asked to add tests.

---

## How to report a bug

Open an issue at https://github.com/fsix7115-arch/AgentForge/issues

Use the bug report template. The most useful things to include:

- What you ran (exact command)
- What you expected to happen
- What actually happened
- Output of `agentforge doctor --verbose`
- Your platform: `uname -a` on Linux/macOS/Termux, or `go env GOOS GOARCH`

If the bug is on a phone, the output of `scripts/termux-check.sh` is very useful.

---

## Good first issues

If you want to contribute but are not sure where to start, these are real gaps:

**Run a real model from a phone.** The phone verification in the README used the `echo` provider, which makes no network call. Running Gemini or Groq from a real phone and reporting the result would close the main open claim.

**Add an OpenAI provider.** The interface is `Agent` — three methods. OpenAI uses the same API shape as Groq (they are compatible). Look at `internal/agent/agent.go` for the Groq implementation; an OpenAI one would be nearly identical.

**Reboot survival.** `docs/TERMUX.md` documents the `termux-boot` approach manually. Writing a script that sets it up and testing it on a real device would be genuinely useful.

**Test on more devices.** The verified phone is one device (Android 15, aarch64). Reports from other devices — different Android versions, different CPUs — help establish what "phone-first" actually means.

**Windows testing.** The binary cross-compiles for `windows/amd64` and `windows/arm64`, but it has not been tested there. Running `go test ./...` and `agentforge doctor` on Windows would confirm or reveal gaps.

---

## Code of conduct

Be direct and honest. Treat people as capable of handling specific, accurate feedback. Avoid vague praise and vague criticism equally.

This project values honest status reporting over marketing. If something is broken, say so. If a claim is unverified, say so. The same standard applies to contributions.
