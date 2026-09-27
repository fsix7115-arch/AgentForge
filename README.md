# AgentForge

**A universal self-hosted AI agent server. Run it anywhere, reach it from
anything.**

```bash
# TODO: real install line once Phase 6 exists
```

> **Status: Phase 0 — research, not code.** There is no software here yet. This
> repository currently holds a research brief, a comparison of 10 existing
> projects, and the prompt that will drive the build. That is deliberate: the
> vision is large enough that coding first would produce something nobody can
> maintain, including the person who asked for it.

---

## The vision

A single server that:

- runs on Android (Termux), Linux, Windows, macOS, a VPS, or a cloud instance
- exposes the same agent through a **terminal**, a **browser**, and an **API**
- keeps memory, tools, and skills as pluggable modules rather than baked in
- stays open source and beginner-maintainable

---

## Phase 1 findings are in the brief

Ten projects were reviewed against this vision, with live data pulled from the
GitHub API rather than from memory: stars, language, license, and last-push
date. The brief is [`docs/RESEARCH.md`](docs/RESEARCH.md).

Four conclusions that should shape any implementation:

1. **This space is crowded and mature.** Ollama at ~182k stars, Hermes at
   ~249k, Open WebUI at ~153k, Langflow at ~155k. There is no obvious gap that
   "one more AI agent" fills. AgentForge has to earn its place or it will be a
   hobby project nobody installs.

2. **The three licenses to copy are MIT and Apache-2.0.** Open WebUI and
   Flowise both report `NOASSERTION` from the GitHub API, which is a warning
   sign, not a permission grant. A project asking beginners to run it should
   be unambiguous about licensing.

3. **Goose and Open Interpreter both rewrote themselves in Rust.** Both started
   in Python. That is a large, public admission that the original
   architecture did not scale. It is worth understanding why before repeating
   it.

4. **There is no single winner and that is the actual opportunity.** Terminal
   tools, browser UIs, orchestration GUIs, and model runtimes each own one
   layer. The space between them — one agent, many surfaces, one memory — is
   where something new could fit.

**Correction, added after Phase 2.** Point 4 was wrong. The layer-between
position is already occupied by Hermes itself. Phase 2 also found four
projects this list missed. See [`VALIDATION.md`](VALIDATION.md).

## Phase 2 — the position, narrowed

Phase 1 ended with three candidate positions. Phase 2 tested all three against
the market. Two are already occupied and one survives.

**A gap search turned up four more projects Phase 1 missed**, including
[Gormes](https://gormes.ai/) — one ~55.6 MB Go binary, MIT, Linux/macOS/
Windows/Android, with chat, memory, dashboards and gateways. That is the
"easiest install" position, already shipped. **Hezo** (GPL-3.0) does agent
teams. Two other single-binary projects, `hermes-agent-rust` and `Hector`,
made the same pitch and have both been quiet for months.

So:

| Position | Verdict |
|---|---|
| Easiest-to-install agent server | **Taken** — Gormes, and Ollama before it |
| One agent, many surfaces, one memory | **Taken** — that is Hermes, at 249k stars |
| **Something for a phone** | **The remaining one** |

> AgentForge is the AI agent server that runs on a phone — installed, updated
> and tested on the phone, reachable from the phone — and works the same on a
> VPS when the phone is not it.

The MVP is six things: a binary that builds on `aarch64-linux-android`, a
loopback HTTP server, one model provider, one messaging interface, SQLite
state in one deletable file, and an offline `doctor` command.

No browser UI. No plugin marketplace. No auth. No database. No Docker-first.
No visual builder.

**The test that decides it:** someone must be able to run the build on a real
Android phone in Termux without asking the author a question.

Full reasoning in [`docs/VALIDATION.md`](docs/VALIDATION.md).

## Roadmap

| Phase | Status |
|---|---|
| 1. Market and project research | **done** — [`docs/RESEARCH.md`](docs/RESEARCH.md) |
| 2. Problem validation and MVP | **done** — [`docs/VALIDATION.md`](docs/VALIDATION.md) |
| 3. Architecture | not started |
| 4. Learning roadmap | not started |
| 5. Development roadmap | not started |
| 6. Implementation | not started |
| 7. Quality check | not started |
| 8. Open-source release | not started |

## The prompt

The 8-phase prompt that drives this build is in
[`docs/AGENTFORGE_PROMPT.md`](docs/AGENTFORGE_PROMPT.md). It is written to be
handed to any capable AI agent, on this repository or a fresh machine.

## License

MIT. Chosen because it is the most permissive widely-understood license, and
because a beginner-maintained project should not have copyleft obligations
that are easy to violate by accident.

## Contributing

Not yet. See Phase 8.

---

*Generated as part of an agent research workflow. Every number in
`docs/RESEARCH.md` came from the GitHub API during the session that produced
this file, not from memory or from marketing copy.*
