# Quality and Testing

This document covers how to run the tests, what they cover, how to add new ones, and what to watch for when working on this codebase.

---

## Running the tests

```bash
# All tests, all packages
go test ./...

# With the race detector (run this before any PR)
go test -race ./...

# Static analysis
go vet ./...

# A single package
go test ./internal/store/...

# Verbose output
go test -v ./...

# A specific test
go test -v -run TestStore_AtomicWrite ./internal/store/...
```

All tests are offline. None of them make network calls or require an API key. The `echoAgent` is the only provider the tests use.

---

## What each test file covers

### `internal/config/config_test.go` — 10 tests

Configuration resolution: are defaults right, do env vars override defaults, do flags override env vars?

Key tests:
- `TestDefaults` — the zero-value config has the right defaults
- `TestLoad_EnvOverride` — `AGENTFORGE_PROVIDER=groq` switches the provider
- `TestLoad_ProviderSwitchesKeyEnv` — `--provider groq` switches to `GROQ_API_KEY`
- `TestLoad_FlagBeatsEnv` — a flag wins over an env var
- `TestValidate_*` — invalid configs are rejected with useful messages

The key-switching tests were added after a real bug: `--provider groq` was switching the provider but not the key variable. The test for this lives in `TestLoad_ProviderSwitchesKeyEnv`.

### `internal/agent/agent_test.go` — 9 tests

The agent interface and the echo provider. No real model calls.

Key tests:
- `TestEcho_Offline` — the echo provider reports `Offline() == true`
- `TestEcho_Reply` — the echo reply includes the turn count
- `TestNew_UnknownProvider` — `agent.New` with an unknown provider returns an error
- `TestNew_MissingKey` — `agent.New` with gemini and no key returns a useful error
- `TestHistoryText_Truncation` — `historyText` truncates from the oldest side

### `internal/store/store_test.go` — 9 tests

Persistence: correctness of read/write, atomicity, turn trimming, concurrency.

Key tests:
- `TestStore_Append` — a turn is saved and can be loaded
- `TestStore_TurnLimit` — old turns are dropped when the limit is reached
- `TestStore_Clear` — clear deletes the file, not just its contents
- `TestStore_AtomicWrite` — interrupted writes leave the file intact (tested by writing to a temp location)
- `TestStore_ConcurrentAppend` — multiple goroutines appending simultaneously do not corrupt the file (this is the race-detector test)
- `TestStore_NoSession` — loading from a nonexistent file returns `ErrNoSession`, not an error

### `internal/server/server_test.go` — 12 tests

HTTP layer: correct status codes, correct JSON shapes, correct error handling.

Key tests:
- `TestHealth` — `/health` returns 200 with the right fields
- `TestChat_OK` — a valid POST `/chat` returns 200 with `reply`, `provider`, `model`, `turns`, `elapsed`
- `TestChat_EmptyMessage` — empty message returns 400
- `TestChat_WhitespaceMessage` — a message of only spaces returns 400
- `TestChat_BadJSON` — malformed JSON returns 400
- `TestClear_OK` — POST `/clear` returns 200 and clears history
- `TestNotFound` — unknown paths return 404
- `TestContentTypeHeader` — all responses set `Content-Type: application/json`

### `internal/doctor/doctor_test.go` — 11 tests

The offline diagnostic tool. Key requirement: `doctor` must never make a network call in the default path.

Key tests:
- `TestDoctor_NoNetworkCall` — `Run` with `probeNetwork=false` makes no outbound connections (verified by binding a fake server and checking it was not called)
- `TestDoctor_KeyRedaction` — the API key value never appears in doctor output
- `TestDoctor_PortConflict` — a port that is already bound shows a failure, not a panic
- `TestDoctor_NoSession` — a missing session file is reported as "not created yet", not as an error
- `TestDoctor_AllPass` — a fully configured echo setup returns `Report.OK == true`

---

## How to add new tests

Go's testing package does not need a framework. A test is a function named `TestXxx` in a `_test.go` file in the same package:

```go
func TestMyThing(t *testing.T) {
    got := myFunction("input")
    want := "expected output"
    if got != want {
        t.Errorf("myFunction(%q) = %q, want %q", "input", got, want)
    }
}
```

**For server tests**, use `httptest`:

```go
func TestMyEndpoint(t *testing.T) {
    // build a server with the echo provider so no key is needed
    cfg := config.Defaults()
    cfg.Provider = "echo"
    ag := &echoAgent{}  // or agent.New(cfg) after setting echo
    st := store.New(t.TempDir()+"/test.db", 40)
    srv := server.New(cfg, ag, st, slog.Default())

    rec := httptest.NewRecorder()
    req := httptest.NewRequest(http.MethodPost, "/chat",
        strings.NewReader(`{"message":"hello"}`))
    req.Header.Set("Content-Type", "application/json")

    srv.Handler().ServeHTTP(rec, req)

    if rec.Code != http.StatusOK {
        t.Fatalf("status %d, want 200", rec.Code)
    }
}
```

**For store tests**, use `t.TempDir()` to get a directory that is cleaned up after the test:

```go
func TestMyStoreFeature(t *testing.T) {
    path := filepath.Join(t.TempDir(), "test.db")
    st := store.New(path, 40)
    // ...
}
```

**Rules:**
- Tests must pass with `CGO_ENABLED=0` (required for Android cross-compile)
- Tests must pass with `-race` (no data races)
- Tests must not make network calls
- Use `t.TempDir()` for temporary files, not `/tmp` directly

---

## Common bugs and debugging steps

### `401 Invalid API Key`

**Symptom:** Real provider returns 401.

**Diagnosis:** `agentforge doctor --verbose` — look at the `provider-key` line. It shows which variable is being read and how many characters it found.

```bash
agentforge doctor --verbose --provider groq
# [ok  ] provider-key  GROQ_API_KEY is set (51 chars, value hidden)
```

If it says `GEMINI_API_KEY is not set` when you are using Groq, the provider switch did not change the key variable. This was a real bug, now fixed by the `providerDefaults` table and tested in `config_test.go`.

### `no space left on device` during build on a phone

**Symptom:** Build fails with a linker error about no space.

**Diagnosis:** `df -h $HOME` in Termux. A Go build needs ~1.5 GB free.

**Fix:** `pkg clean` to remove cached packages. Remove old binaries. Restart Termux to release held memory.

### Port already in use

**Symptom:** `agentforge serve` fails immediately, or `doctor` reports the port as taken.

**Diagnosis:** `pkill -f agentforge` — another instance is running. On a phone this happens if you ran `serve` and then closed the Termux window without Ctrl-C.

### Session file corrupt

**Symptom:** `doctor` reports the session as unreadable. `chat` produces an error about JSON parsing.

**Diagnosis:** `cat ~/.agentforge/agentforge.db` — is the JSON valid? A corrupt file means a write was interrupted before the atomic rename completed (unusual but possible if the device ran out of storage mid-write).

**Fix:** `agentforge clear` or `rm ~/.agentforge/agentforge.db`. History is gone, but the binary continues.

### `cannot execute binary file: Exec format error` on phone

**Symptom:** The binary built fine but the phone refuses to run it.

**Diagnosis:** `uname -m` on the phone. If it says `armv7l`, the phone is 32-bit. The binary is `arm64` (64-bit). They are not compatible.

**Fix:** A 32-bit Android phone cannot run the binary. There is no fix on the same device; use a different phone.

### Tests pass but binary fails on phone

**Symptom:** `go test ./...` passes on a laptop, but the same code behaves differently on a phone.

**Diagnosis:** Run `scripts/termux-check.sh` and note which specific check fails. The most common causes:
- File system differences: Termux's home is at `/data/data/com.termux/files/home`, not `/home/user`
- Storage permissions: `termux-setup-storage` may be needed for external storage access
- Android process killing: the OS kills background processes on battery pressure

---

## Security concerns

**API keys:** Keys are never stored. The config holds the variable name, not the value. `doctor` prints the character count, not the value. If you add a new feature that logs config values, audit it to ensure the key value cannot leak.

**Session file permissions:** The session file is `0o600`. A test that creates a session file should check the permissions if it touches the `saveLocked` path.

**Body size:** The `/chat` handler caps the body at 1 MB. If you add a new handler that reads a body, add the same cap: `r.Body = http.MaxBytesReader(w, r.Body, 1<<20)`.

**Loopback assumption:** The security model assumes loopback. If you add a feature that makes outbound calls or that listens on a non-loopback address, document the change explicitly.

**No CORS:** The server intentionally sends no `Access-Control-Allow-Origin` header. Do not add one without understanding the security implications.

---

## Performance concerns for mobile

**Binary size:** The binary is about 11 MB. Large transitive dependencies (logging frameworks, ORM layers, crypto libraries) grow this quickly. Check binary size after adding a dependency: `go build -o /tmp/af ./cmd/agentforge && ls -lh /tmp/af`.

**Memory:** The default turn limit is 40 turns at ~200 bytes each — about 8 KB for the session. The `historyText` function caps the prompt at 6000 characters before sending to the provider. Do not increase this cap without testing on a phone; a 64 KB prompt on a 2 GB RAM phone is meaningfully different from a laptop.

**Startup time:** The binary starts in under 100ms. `doctor` and `chat` are interactive commands; slow startup is user-visible. Avoid init-time work (reading files, making network calls) that is not needed for every command.

**Atomic writes:** The temp-file-then-rename pattern is slower than in-place writes. It is the right choice anyway. On a phone with flash storage, the extra write is not meaningfully slower, and the correctness guarantee is worth it.

---

## How to test on Termux

The complete verification is `scripts/termux-check.sh`. For manual testing:

```bash
# install if needed
pkg install git golang

# build
git clone https://github.com/fsix7115-arch/AgentForge
cd AgentForge
go test ./...
go build -o agentforge ./cmd/agentforge

# check
./agentforge doctor --verbose

# offline chat
./agentforge chat --provider echo "hello"

# HTTP server
./agentforge serve --provider echo &
sleep 1
curl -s localhost:8787/health
curl -s localhost:8787/chat -d '{"message":"test"}'
kill %1

# with a real key
export GEMINI_API_KEY=your_key_here
./agentforge chat "what model are you?"
```

If any step fails, open an issue with the full output. Include the output of `uname -a` and `go version`.
