# Development Roadmap

This is the week-by-week plan used to build AgentForge. It is written after the fact, but reflects the actual sequence of decisions and outputs.

Use this as a template if you want to build something similar from scratch, or as context for understanding why the project is the way it is.

---

## Overview

```
Week 1  — Environment setup and research
Week 2  — Core CLI skeleton
Week 3  — HTTP server
Week 4  — Providers (echo, gemini, groq)
Week 5  — Testing
Week 6  — Phone verification
Week 7  — Release preparation
Week 8  — Open-source launch
```

Eight weeks assumes roughly 2-4 hours per day. Each week has a goal, specific tasks, expected outputs, and success criteria. "Success criteria" means: how do you know this week is done? If you cannot answer that question, the week is not done.

---

## Week 1: Environment setup and research

**Goal:** Understand the landscape before writing a line of code.

**Tasks:**
1. Install Go 1.22+ (`go version`)
2. Install Git and configure it (`git config --global user.name`, `user.email`)
3. Read at least five existing AI agent projects on GitHub — what do they do, who are they for, what do they miss?
4. Identify the gap: what does nobody do well?
5. Write down the positioning in one sentence

**Expected output:**
- Working Go installation that can build a hello-world binary
- A short document (can be a text file) listing what you looked at and what gap you found
- One-sentence positioning: "AgentForge is the AI agent server that [specific claim]"

**Success criteria:**
- `go build` produces a binary
- You can name at least three existing projects and explain specifically why each one does not own your chosen position
- You have written the positioning down, not just thought it

**Notes:**
This is the week that [docs/RESEARCH.md](RESEARCH.md) documents. Do not skip it. Building something that already exists well is waste. The research step is what turns "I want to build an AI project" into "I am building the specific thing nobody else is building for this specific reason."

---

## Week 2: Core CLI skeleton

**Goal:** A binary that parses commands and configuration correctly.

**Tasks:**
1. Initialize the Go module (`go mod init`)
2. Create `cmd/agentforge/main.go` with subcommand dispatch
3. Create `internal/config/config.go` with resolution order: defaults → env → flags
4. Create `internal/version/version.go`
5. Wire up `agentforge version` and `agentforge help`
6. Write tests for config: defaults, environment override, flag override, validation

**Expected output:**
- `agentforge version` prints a version string
- `agentforge help` prints usage
- `agentforge --provider invalid` exits with a useful error
- Config tests pass: `go test ./internal/config/...`

**Success criteria:**
- `go test ./internal/config/...` passes with at least 8 tests
- Unknown flags exit non-zero with a readable error
- The resolution order is tested: a flag beats an env var beats a default

**Notes:**
The provider defaults table (`gemini → GEMINI_API_KEY`, `groq → GROQ_API_KEY`) belongs here. One real bug was found at this stage: switching `--provider groq` did not switch the key variable, producing a 401 later that looked like a bad key. Catching this in config tests, before the provider is even built, is the right time.

---

## Week 3: HTTP server

**Goal:** A running HTTP server with three endpoints.

**Tasks:**
1. Create `internal/server/server.go` with:
   - `GET /health`
   - `POST /chat` (stub — returns a placeholder for now)
   - `POST /clear`
   - `GET /` (endpoint discovery)
2. Inject the agent via interface (so tests can use the echo provider without a key)
3. Add request logging
4. Add graceful shutdown on SIGINT/SIGTERM
5. Write server tests using `httptest.NewRecorder`

**Expected output:**
- `agentforge serve --provider echo` starts and responds to requests
- `curl -s localhost:8787/health` returns JSON
- Server shuts down cleanly on Ctrl-C
- Server tests pass: `go test ./internal/server/...`

**Success criteria:**
- `go test ./internal/server/...` passes with at least 10 tests
- Empty message (`{"message":""}`) returns 400, not 500
- `/health` includes the provider name and offline status

**Notes:**
The `Handler()` method (returns `http.Handler` without starting the server) is what makes tests clean. Tests use `httptest.NewRecorder` and never bind a port. The server itself calls `Serve` only in production.

Body size limit (1 MB) goes here. On a phone, an accidental `curl` with a file body should not fill memory.

---

## Week 4: Providers

**Goal:** Real AI replies from gemini and groq; offline echo for testing.

**Tasks:**
1. Create `internal/agent/agent.go` with the `Agent` interface
2. Implement `echoAgent` — returns immediately, no network, no key
3. Implement `geminiAgent` — POST to Google Generative Language API
4. Implement `groqAgent` — POST to Groq OpenAI-compatible API
5. Test against real APIs (not just mocks) — gemini and groq both
6. Write agent tests with the echo provider
7. Wire providers into the CLI (`--provider` flag → `agent.New`)

**Expected output:**
- `agentforge chat --provider echo "hello"` works offline
- `agentforge chat "hello"` with a valid Gemini key returns a real reply
- `agentforge chat --provider groq "hello"` with a valid Groq key returns a real reply
- `go test ./internal/agent/...` passes

**Success criteria:**
- `go test ./internal/agent/...` passes with at least 8 tests
- A real Gemini reply is shown in the repo history or the RESEARCH doc
- A real Groq reply is shown
- `--provider groq` correctly reads `GROQ_API_KEY`, not `GEMINI_API_KEY`

**Notes:**
The browser user-agent workaround for Groq is discovered here. Cloudflare's bot detection on the Groq API rejects the default Go user-agent with error 1010. The fix is a single header; the test for this behavior is in `agent_test.go`.

`historyText` (flatten turns to a single string) is a pragmatic simplification. The correct approach for Gemini is the multi-turn `contents` array. The simplification works and is faster to implement. Revisit in a later phase.

---

## Week 5: Storage and testing

**Goal:** Persistent conversation history and a complete test suite.

**Tasks:**
1. Create `internal/store/store.go` with:
   - Atomic JSON writes (temp file → rename)
   - Turn limit and trimming
   - `Load`, `Append`, `Clear`, `Stats`
   - Mutex for concurrent access
2. Create `internal/doctor/doctor.go` with offline checks:
   - Version and platform
   - Config values
   - Data directory write test
   - Session stats
   - Provider key presence (never print the value)
   - Port availability
3. Wire store into server and CLI
4. Write tests for store and doctor
5. Run the full suite: `go test -race ./...`

**Expected output:**
- Conversation history persists between `agentforge chat` calls
- `agentforge doctor` passes offline (no network call)
- `go test -race ./...` passes all 51 tests

**Success criteria:**
- All five packages have tests: `config`, `agent`, `store`, `server`, `doctor`
- Total test count is at least 45
- The race detector (`-race`) finds no issues
- `agentforge clear` deletes the session file and `doctor` reports it as not-created

**Notes:**
The atomic write is the most important correctness property in this codebase. Android kills processes without warning. A partial write corrupts the JSON file. Write-to-temp-then-rename means a killed process leaves either the old file or the new one, never a partial one.

File permissions: `0o600` for the session file, `0o700` for the data directory. On a shared or rooted phone this matters.

---

## Week 6: Phone verification

**Goal:** Confirm the central claim. The binary must run on a real Android phone.

**Tasks:**
1. Cross-compile for `android/arm64`:
   ```bash
   GOOS=android GOARCH=arm64 CGO_ENABLED=0 go build -o agentforge-android ./cmd/agentforge
   ```
2. Write `scripts/termux-check.sh` — the one-command verification script
3. Transfer to a phone or borrow one
4. Run the check script
5. Document the exact output in README.md under "Verified on a real phone"
6. Note what was proven and what was not proven

**Expected output:**
- The check script passes on at least one real device
- README.md includes the actual output from that device
- `scripts/termux-check.sh` is committed and tested

**Success criteria:**
- A real phone shows "THIS PHONE CAN RUN AGENTFORGE" from the check script
- The device details (Android version, CPU, Go version) are documented
- The section in README.md honestly distinguishes what was proven (binary runs, doctor passes, HTTP server responds) from what was not proven (real model from a phone)

**Notes:**
This week is the one the whole project exists for. Until a real phone runs it, the "phone-first" claim is a goal, not a fact. The check script exists specifically to make the evidence reproducible: anyone with a phone can run it and contribute a data point.

The echo provider is used for verification, not Gemini or Groq. The binary running and the server answering is what this check proves. Running a real model from a phone is the next open claim.

---

## Week 7: Release preparation

**Goal:** Someone else can build and run this without help.

**Tasks:**
1. Write `docs/TERMUX.md` — the phone install guide
2. Write `docs/ARCHITECTURE.md` — how it works and why
3. Verify cross-compilation for all seven targets:
   - `linux/amd64`, `linux/arm64`, `linux/386`
   - `android/arm64`
   - `darwin/arm64`
   - `windows/amd64`, `windows/arm64`
4. Write `.github/workflows/release.yml` — automated release builds on git tag
5. Update README.md with honest status, verified output, and roadmap
6. Run `go vet ./...` and fix any issues

**Expected output:**
- All seven cross-compile targets succeed
- README.md has the verified phone output
- Release workflow produces binaries on a `v*` tag push

**Success criteria:**
- `GOOS=android GOARCH=arm64 CGO_ENABLED=0 go build ./cmd/agentforge` succeeds
- `go vet ./...` produces no output
- README.md tells the truth about what is done and what is not

**Notes:**
`android/amd64` is not a target. It requires cgo (`GOOS=android GOARCH=amd64` fails with `CGO_ENABLED=0`), and the phone this project targets is `arm64`. This is documented rather than silently omitted.

The release workflow uses `softprops/action-gh-release` which handles creating the GitHub release and attaching artifacts. Tests run only on `linux/amd64` to avoid slow cross-compilation in CI.

---

## Week 8: Open-source launch

**Goal:** The project is ready for external contributors.

**Tasks:**
1. Add `CONTRIBUTING.md`
2. Add `.github/ISSUE_TEMPLATE/bug_report.md`
3. Add `.github/ISSUE_TEMPLATE/feature_request.md`
4. Add `.github/PULL_REQUEST_TEMPLATE.md`
5. Write `docs/QUALITY.md`
6. Write `docs/LEARNING_ROADMAP.md`
7. Write `docs/DEVELOPMENT_ROADMAP.md` (this file)
8. Audit the README: does it accurately describe the current state?
9. Push all docs
10. Announce: link to the phone verification section specifically

**Expected output:**
- All documentation files exist and are accurate
- The issue templates guide reporters to give useful information
- A first-time contributor can find everything they need in `CONTRIBUTING.md`

**Success criteria:**
- Someone not on the project can clone the repo, build it, run the tests, and open a PR without asking any questions
- The README does not overclaim. It says what is done, what is not done, and what is the biggest open question
- The phone verification section has real output from a real device

**Notes:**
The most useful thing anyone can do for this project is run `scripts/termux-check.sh` on a real Android phone. The launch message should make that specific request, not a general "try it out."

---

## What comes after week 8

These are not scheduled. They are the natural next steps, listed in rough priority order.

**Run a real model from a phone.** The verified phone test used the `echo` provider. Running Gemini or Groq from a phone is a different claim and has not been confirmed. This is the largest open question.

**Release binaries.** Currently the install requires a Go toolchain. Publishing release binaries via the GitHub Actions workflow closes that gap.

**Add an OpenAI provider.** The interface is already defined. The Groq implementation is nearly identical to what an OpenAI provider would look like. This is a beginner-friendly contribution.

**Reboot survival.** The `termux-boot` approach is documented but not automated. A setup command (`agentforge install-boot`) would make this reliable.

**Streaming replies.** Long responses from a phone's mobile connection can take 15+ seconds. Server-sent events or chunked JSON would improve the experience.

**Multi-session support.** The current store has exactly one session (`"default"`). The `/chat` endpoint already accepts a `session` field (it is ignored). Honoring it is the path to multi-session.

**A second storage backend.** SQLite is the right long-term answer. The `Store` interface is the seam. A SQLite implementation alongside the JSON one, selectable by config, would prove the interface is correct.
