# Phase 1 — Market and project research

**Date: 2026-09-27.** Every number below came from the GitHub REST API during
the session that wrote this file. Stars and last-push dates move, so treat
them as a snapshot. Where a repo has moved or been renamed, the current
location is given rather than the one in the original brief.

A note on method: three of the ten repositories in the brief returned HTTP 301
when queried, meaning they had been moved. The current paths are used below.
Two report `NOASSERTION` for license, which means GitHub could not
automatically detect a license file — it is not a permissive grant and is
treated as "unknown" throughout.

---

## The comparison

| Project | Stars | Language | License | Last push | What it is |
|---|---:|---|---|---|---|
| **Hermes Agent** | 249,340 | Python | MIT | 2026-09-27 | Personal agent with terminal, browser, Telegram |
| **Ollama** | 181,798 | Go | MIT | 2026-09-26 | Local model runtime |
| **Langflow** | 155,291 | Python | MIT | 2026-09-27 | Visual flow builder for LLM apps |
| **Open WebUI** | 153,315 | Python | *unknown* | 2026-09-26 | Chat UI for local models |
| **Open Interpreter** | 68,451 | Rust | Apache-2.0 | 2026-09-27 | Runs code on the local machine |
| **Goose** | 54,700 | Rust | Apache-2.0 | 2026-09-25 | Block's agentic coding agent |
| **Flowise** | 55,486 | TypeScript | *unknown* | 2026-08-13 | Visual LLM workflow builder |
| **Aider** | 49,214 | Python | Apache-2.0 | 2026-05-22 | Pair programmer, git-aware |
| **LibreChat** | 44,986 | TypeScript | MIT | 2026-09-27 | Multi-provider chat UI |
| **Continue** | 36,042 | TypeScript | Apache-2.0 | 2026-09-27 | IDE assistant |

Repo paths: `NousResearch/hermes-agent`, `ollama/ollama`,
`langflow-ai/langflow`, `open-webui/open-webui`,
`openinterpreter/openinterpreter` (formerly `OpenInterpreter/…`),
`aaif-goose/goose` (formerly `block/goose`), `FlowiseAI/Flowise`,
`Aider-AI/aider`, `LibreChat-AI/LibreChat` (formerly `danny-avila/…`),
`continuedev/continue`.

**OmniRoute could not be reviewed.** It is present on this machine as
`~/.omniroute` but is not on GitHub, so it has no public repository, license,
or documentation to assess. It was excluded rather than described from
guesswork.

---

## Project by project

### Hermes Agent — `NousResearch/hermes-agent`
- **Does:** a personal agent that runs on the user's own machine and is
  reachable from a terminal, a browser, and messaging platforms.
- **Strengths:** the closest existing match to the AgentForge vision. Multiple
  interfaces over one agent. Large, very recent, actively pushed.
- **Weaknesses:** Python, and Python is the runtime bottleneck for a
  long-lived server. 44k open issues suggests fast growth outpacing triage.
  MIT, so reuse is legal.
- **Stack:** Python.
- **Install difficulty:** moderate. Requires a Python environment and provider
  keys.
- **Lesson for AgentForge:** the multi-interface-over-one-agent idea is proven,
  not speculative. Competing against it head-on on features is a losing move.
  The lesson is to compete on *install simplicity* — one binary, no Python.

### Ollama — `ollama/ollama`
- **Does:** downloads and serves local models with a single command.
- **Strengths:** Go, so a single static binary. The install experience everyone
  else is measured against. 181k stars with obvious utility.
- **Weaknesses:** it is a runtime, not an agent. It does not converse, remember,
  or act. No agent loop.
- **Stack:** Go.
- **Install difficulty:** lowest of the ten. One command.
- **Lesson:** distribution is the product. `curl … | sh` and a local HTTP port
  won this category. AgentForge cannot compete on runtime, but it can compete
  on this axis by being similarly trivial to start.

### Langflow — `langflow-ai/langflow`
- **Does:** drag-and-drop canvas for building LLM pipelines visually.
- **Strengths:** MIT, Python, extremely active, 155k stars. Makes complex
  orchestration legible to non-programmers.
- **Weaknesses:** a web app first. Heavy dependency install, browser required,
  not usable from a phone terminal. Visual wiring does not scale to a large
  codebase.
- **Stack:** Python (backend) + TypeScript (frontend).
- **Install difficulty:** high. Many dependencies.
- **Lesson:** a visual layer attracts people who cannot code, and repels people
  who can. AgentForge should not try to be both.

### Open WebUI — `open-webui/open-webui`
- **Does:** a polished chat interface for self-hosted and local models.
- **Strengths:** 153k stars, extremely active, works with Ollama out of the
  box. Excellent onboarding.
- **Weaknesses:** **license reports as `NOASSERTION`** — GitHub could not
  identify a license. That is a real adoption blocker for a project asking
  people to self-host it in production. Python, browser-only.
- **Stack:** Python + Svelte.
- **Install difficulty:** moderate via Docker, hard otherwise.
- **Lesson:** licensing clarity is not a footnote. Check it before you
  recommend a project, and get it right in your own.

### Open Interpreter — `openinterpreter/openinterpreter`
- **Does:** lets an LLM write and run code on the local machine.
- **Strengths:** real local execution, which is what makes it useful. Apache-2.0.
- **Weaknesses:** **rewritten in Rust** from an original Python codebase. That is
  a large engineering decision, and the Python version and its community are
  effectively gone.
- **Stack:** Rust.
- **Install difficulty:** moderate.
- **Lesson:** if you rewrite, you lose the contributors who knew the code. A
  beginner-maintained project should pick a stack and stay there.

### Goose — `aaif-goose/goose`
- **Does:** Block's open-source agentic coding agent.
- **Strengths:** backed by a large company, Apache-2.0, actively developed.
  Strong at multi-step coding tasks.
- **Weaknesses:** company priorities will eventually diverge from community
  needs. Also rewritten in Rust. Coding-focused, not general-purpose.
- **Stack:** Rust.
- **Install difficulty:** moderate.
- **Lesson:** corporate backing brings resources and brings a roadmap you do
  not control.

### Flowise — `FlowiseAI/Flowise`
- **Does:** visual builder for agent and RAG workflows, OpenWebUI-compatible.
- **Strengths:** 55k stars, large template gallery, very popular.
- **Weaknesses:** **license `NOASSERTION`**. Last push 2026-08-13, which is
  noticeably older than the others. TypeScript.
- **Stack:** TypeScript.
- **Install difficulty:** moderate via Docker.
- **Lesson:** slow repository activity is a warning. Check `pushed_at`, not just
  stars.

### Aider — `Aider-AI/aider`
- **Does:** pair programmer that edits code and commits to git.
- **Strengths:** the most focused tool of the ten. Excellent git integration.
  Apache-2.0. Simple mental model — one job, done well.
- **Weaknesses:** last push 2026-05-22, over four months before this review.
  That is a significant gap. Terminal-only, coding-only.
- **Stack:** Python.
- **Install difficulty:** low.
- **Lesson:** the smallest project here is the most focused, and the two
  projects with the longest gaps since last push are both Python. Python
  agent projects in this space are not being kept up.

### LibreChat — `LibreChat-AI/LibreChat`
- **Does:** multi-provider chat UI with agents, plugins, and conversation
  history.
- **Strengths:** MIT, very active, supports many providers, the closest thing to
  a "one UI, all models" experience.
- **Weaknesses:** browser-first and heavy. Overlaps heavily with Open WebUI, so
  choosing between them is mostly taste. TypeScript.
- **Install difficulty:** high. Multi-service, needs a database.
- **Lesson:** chat UIs are the crowded part of this market. Do not build one.

### Continue — `continuedev/continue`
- **Does:** assistant for IDEs, with autocomplete and chat.
- **Strengths:** Apache-2.0, very active, tight IDE integration, 36k stars.
- **Weaknesses:** bound to the IDE form factor. Not a server, not multi-surface.
  Narrow audience.
- **Stack:** TypeScript.
- **Install difficulty:** low, via extension.
- **Lesson:** deep integration with one surface beats shallow support for many.

---

## What AgentForge should learn — the short list

1. **Distribution beats features.** Ollama's single binary is the reason it is
   everywhere. Install simplicity is a feature.
2. **Pick a stack and never rewrite.** Both Rust rewrites lost their
   communities. Both Python projects with stale activity are struggling.
3. **License clarity or nothing.** Two of the ten have no detectable license.
   Do not copy that.
4. **Do not build another chat UI.** LibreChat, Open WebUI, and Flowise already
   saturate that space.
5. **Be narrow early.** Aider and Continue are strong precisely because they
   do one thing. AgentForge's MVP must be narrower than its vision.
6. **Check `pushed_at` before you admire the stars.** Two projects here have
   significant gaps.

## The uncomfortable conclusion

There is no gap that "another AI agent server" fills. This space has ten mature
projects, two of them past 180k stars, and the categories are crowded from the
model runtime up through the UI layer.

The viable versions of AgentForge are narrow:

- **the easiest-to-install agent server that exists** (compete with Ollama's
  install experience, not with Hermes's features)
- **one agent, many surfaces, one memory** (the gap between the layers)
- **something for a phone** — Termux-friendly, which none of the ten emphasise

If none of those three can be stated in one sentence that is more compelling
than any existing project's own description, the honest answer is that the
project should not be built. That question is Phase 2.
