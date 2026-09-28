# Learning Roadmap

This project exists in a specific technology stack. If you want to understand it, contribute to it, or build something similar, this page maps out what to learn and in what order.

The list is honest about difficulty and realistic about time. "You can learn X in a weekend" claims are skipped. Real learning takes real time.

---

## How to use this

Read the whole list first, then pick the topic that matches where you are. If you are completely new to programming, start with Git and Python basics. If you already write Python, jump to Go or APIs.

You do not need to finish everything before contributing. The echo provider tests work without any API key; `go test ./...` runs in a fresh checkout with just Go installed.

---

## 1. Git and GitHub

**Why it matters:** Every line of code in this project is managed with Git. Contributing requires knowing how to clone, branch, commit, and open a pull request. There is no workaround.

**What to learn:**
- Clone, add, commit, push
- Branches: create, switch, merge
- Pull requests: fork, branch, open PR
- Reading a diff

**Free resources:**
- https://git-scm.com/book/en/v2 — the official Pro Git book, free online, Chapters 1-3 cover everything you need
- https://learngitbranching.js.org — interactive browser exercises, good for branching concepts
- https://docs.github.com/en/get-started/quickstart — GitHub's own quickstart

**Beginner project:** Fork this repo, add your name to a new file (`contributors.md`), and open a pull request.

---

## 2. Python basics

**Why it matters:** Python is the language most AI/ML tutorials use. Even if AgentForge is written in Go, most examples for calling the Gemini or Groq APIs are in Python first.

**What to learn:**
- Variables, types, functions
- Lists, dicts, loops
- File I/O: reading and writing files
- `requests` library: making HTTP calls
- Environment variables: `os.environ`

**Free resources:**
- https://docs.python.org/3/tutorial/ — the official tutorial, dry but complete
- https://automatetheboringstuff.com — free online book, very practical
- https://realpython.com/python-beginner-tips/ — good orientation

**Beginner project:** Write a Python script that calls the Gemini API directly and prints the response. No framework, just `requests` and a key.

---

## 3. Go basics

**Why it matters:** AgentForge is written in Go. Reading the source and contributing requires knowing Go.

**What to learn:**
- Types, functions, structs
- Error handling: `if err != nil`
- Packages and modules
- The standard library: `os`, `fmt`, `encoding/json`, `net/http`
- `go build`, `go test`, `go vet`
- Interfaces: how `Agent` works in this project

**Free resources:**
- https://go.dev/tour/ — the official tour, browser-based, takes 3-5 hours
- https://go.dev/doc/effective_go — the style guide, worth reading after the tour
- https://gobyexample.com — short, concrete examples for every language feature

**Beginner project:** Build the echo provider by yourself without looking at `internal/agent/agent.go`. It takes a string and returns a string. That is the whole thing.

**Specific to this project:** Read `internal/store/store.go`. The atomic write pattern (write to `.tmp`, then rename) is a real technique you will use in other projects.

---

## 4. APIs and REST

**Why it matters:** The server in this project is an HTTP API. The providers (Gemini, Groq) are HTTP APIs. Understanding REST is understanding most of what this project does.

**What to learn:**
- HTTP methods: GET, POST
- Status codes: 200, 400, 404, 500, 502
- JSON: encoding and decoding
- Headers: `Content-Type`, `Authorization`
- `curl` for manual testing

**Free resources:**
- https://developer.mozilla.org/en-US/docs/Web/HTTP/Overview — MDN's HTTP overview
- https://restfulapi.net — REST concepts without framework noise
- https://httpstatuses.io — reference for status codes

**Beginner project:** Use `curl` to talk to a running AgentForge server. Try `GET /health`, then `POST /chat`, then `POST /clear`. Read what comes back.

```bash
./agentforge serve --provider echo &
curl -s localhost:8787/health
curl -s localhost:8787/chat -d '{"message":"hello"}'
curl -s -X POST localhost:8787/clear
```

---

## 5. FastAPI (Python)

**Why it matters:** If you want to build a similar project in Python, FastAPI is the modern standard. Understanding it gives you context for why Go's `net/http` is designed the way it is.

**What to learn:**
- Route handlers: `@app.get`, `@app.post`
- Request and response models with Pydantic
- Dependency injection
- Running with `uvicorn`

**Free resources:**
- https://fastapi.tiangolo.com/tutorial/ — official tutorial, excellent quality
- https://realpython.com/fastapi-python-web-apis/ — step-by-step with examples

**Beginner project:** Build a minimal chat API in Python/FastAPI that calls the Gemini or Groq API and returns the response. Compare the code size and complexity to AgentForge's `server.go`.

---

## 6. Databases: SQLite and JSON

**Why it matters:** AgentForge stores conversation history as JSON. The comments in `store.go` explain why SQLite was not used for the MVP. Understanding both helps you understand the tradeoff.

**What to learn:**
- JSON: structure, parsing, writing
- SQLite: tables, queries, INSERT/SELECT
- Why atomicity matters (the `.tmp` rename pattern)
- When JSON is enough and when it is not

**Free resources:**
- https://sqlite.org/lang.html — SQLite SQL reference
- https://sqlitebrowser.org — visual SQLite browser, useful for learning
- https://json.org — the JSON spec, short and readable

**Beginner project:** Take AgentForge's JSON session file and write a script (Python or Go) that reads it and prints a formatted transcript of the conversation.

---

## 7. Authentication basics

**Why it matters:** AgentForge deliberately has no auth. Understanding what auth is and why it was left out helps you know when to add it.

**What to learn:**
- API keys: what they are, how they work
- Bearer tokens and the `Authorization` header
- Why loopback (`127.0.0.1`) is a security boundary
- What happens when you expose a service on `0.0.0.0`

**Free resources:**
- https://auth0.com/docs/get-started/identity-fundamentals — foundation concepts
- https://developer.mozilla.org/en-US/docs/Web/HTTP/Authentication — HTTP auth overview

**Beginner project:** Look at how AgentForge passes the API key in `agent.go`. Note that it reads the key from the environment at call time (not at startup). Understand why that matters for a phone where you might edit `.bashrc` and re-run a command.

---

## 8. Docker

**Why it matters:** Docker is how most server software is deployed. Understanding it helps you see why the AgentForge approach (single binary, no dependencies) is specifically designed to avoid needing Docker.

**What to learn:**
- Images and containers
- `docker build`, `docker run`
- A simple Dockerfile for a Go binary
- Why a single static binary is easier to ship than a container

**Free resources:**
- https://docs.docker.com/get-started/ — official Docker getting started
- https://docs.docker.com/language/golang/ — Docker's Go guide specifically

**Beginner project:** Write a `Dockerfile` that builds AgentForge. Notice how much simpler a multi-stage Go build is compared to a Python project.

---

## 9. Linux and terminal basics

**Why it matters:** AgentForge runs in a terminal. Most of its documentation assumes comfort with a shell. On a phone, the terminal is the entire UI.

**What to learn:**
- Navigation: `ls`, `cd`, `pwd`, `mkdir`
- Files: `cat`, `cp`, `mv`, `rm`
- Permissions: `chmod`, why `0600` matters
- Processes: `ps`, `kill`, `pkill`
- Environment variables: `export`, `echo $VAR`
- Pipes and redirection: `|`, `>`, `2>&1`
- `curl` for HTTP requests

**Free resources:**
- https://linuxcommand.org/lc3_learning_the_shell.php — good beginner introduction
- https://tldr.sh — short practical examples for any command
- https://man7.org/linux/man-pages/ — reference when you need the details

**Beginner project:** Run `agentforge doctor --verbose` and understand every line of output. Trace each piece of output back to the code in `internal/doctor/doctor.go`.

---

## 10. Termux (Android)

**Why it matters:** This is the primary deployment target. If you want to run or test AgentForge in its intended environment, Termux is where to do it.

**What to learn:**
- Install from F-Droid (not the Play Store)
- `pkg install git golang`
- How Termux's filesystem differs from a standard Linux: `/data/data/com.termux/files/home`
- Storage permissions: `termux-setup-storage`
- Running a server in the background
- `termux-boot` for survival across reboots

**Free resources:**
- https://wiki.termux.com/wiki/Getting_started — Termux's own wiki
- https://wiki.termux.com/wiki/Development_Environments — setting up Go in Termux
- [`docs/TERMUX.md`](TERMUX.md) in this repo — the project-specific guide

**Beginner project:** Run `scripts/termux-check.sh` on a real Android phone and report the result in a GitHub issue. Success and failure are both valuable.

---

## 11. AI APIs (Gemini, Groq, OpenAI)

**Why it matters:** These are the backends AgentForge connects to. Understanding how the APIs work helps you understand `internal/agent/agent.go` and helps you add a new provider.

**What to learn:**
- API keys: how to get them, how to keep them safe
- Request/response format for each API
- Rate limits and costs
- How to handle errors: timeouts, 401, 429, 500

**Free resources:**
- https://ai.google.dev/gemini-api/docs — Gemini API docs
- https://console.groq.com/docs/openai — Groq's OpenAI-compatible API docs
- https://platform.openai.com/docs/api-reference — OpenAI API reference (Groq is compatible with this shape)

**Beginner project:** Call the Gemini API directly with `curl`, without using AgentForge. Look at the raw JSON response and map it to the parsing code in `geminiAgent.Reply`.

```bash
curl -s "https://generativelanguage.googleapis.com/v1beta/models/gemini-3.8-flash:generateContent" \
  -H "x-goog-api-key: $GEMINI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"contents":[{"parts":[{"text":"hello"}]}]}'
```

---

## 12. LLM fundamentals

**Why it matters:** AgentForge is a client for large language models. Understanding what LLMs are and are not helps you set realistic expectations and make better design decisions.

**What to learn:**
- What a language model does: next-token prediction
- Context windows: why history has to be trimmed
- Temperature: what it controls
- System prompts: what they are, how they influence the model
- What "hallucination" means and why it happens
- Tokens vs characters vs words

**Free resources:**
- https://karpathy.ai/zero-to-hero.html — Andrej Karpathy's video series, starts from scratch
- https://writings.stephenwolfram.com/2023/02/what-is-chatgpt-doing-and-why-does-it-work/ — long but thorough non-technical explanation
- https://github.com/karpathy/nanoGPT — the minimal GPT implementation, ~300 lines of Python

**Beginner project:** Read `historyText` in `internal/agent/agent.go`. Understand why it truncates from the oldest side, and what `maxChars = 6000` is trying to control. Think about what would happen if you sent a 100,000-token history to a model with a 32,000-token context window.

---

## Suggested order

If you are starting from zero: 9 → 1 → 2 → 4 → 6 → 3 → 11 → 7 → 12 → 8 → 10 → 5

If you already write Python: 1 → 3 → 4 → 11 → 12 → 7

If you already write Go: 11 → 12 → 10 → read the source

If you have a phone and want to run it now: 9 → 10 → `docs/TERMUX.md`
