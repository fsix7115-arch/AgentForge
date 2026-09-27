# Phase 2 — Project validation

**Date: 2026-09-27.** Phase 1 reviewed ten projects. This phase was supposed to
confirm that a gap exists. It did not, so it also widened the search — and the
wider search turned up four more projects that Phase 1 missed, three of which
make AgentForge's proposed position considerably harder.

Everything here was checked against the GitHub API or the project's own site
during this session.

---

## The new findings

Phase 1's list came from the original prompt. Searching the space properly
found four more, and they are the ones that matter most:

| Project | Stars | Language | License | Created | Last push |
|---|---:|---|---|---|---|
| **Gormes** | — | Go | MIT | — | live site |
| **Hezo** | 393 | TypeScript | **GPL-3.0** | 2026-03-30 | 2026-09-26 |
| **hermes-agent-rs** | 93 | Rust | MIT | 2026-04-12 | **2026-07-08** |
| **Hector** | 60 | Go | MIT | 2025-09-17 | **2026-05-18** |
| **Vortex** | 1 | Go | *unknown* | 2026-06-03 | 2026-08-17 |
| OpenHands (for scale) | 89,268 | TypeScript | MIT | 2024-03-13 | 2026-09-26 |

### Gormes — the important one

[gormes.ai](https://gormes.ai/) describes itself as: one ~55.6 MB Go binary,
Linux/macOS/Windows/Android, MIT, with chat, memory, dashboards, gateways,
SQLite memory, and an offline doctor command. Installed with
`curl -fsSL ... | sh`. No Python, no Docker, no venv.

**That is all three of AgentForge's proposed positions, already built, already
MIT, already shipping Termux and Android.**

**Gormes has no public GitHub repository.** The site is live and the binary
downloads, but the source is not published. So: no license file to read, no
contributors, no issue tracker, no way to check whether it is maintained or
abandoned. An MIT *claim* on a closed-source binary is legally weaker than an
MIT license file in a repository, and for a project people are asked to run on
machines, that distinction matters.

This does not remove the opportunity — closed source means the design is not
copyable, and the Termux angle in particular is barely covered. But it does
mean the "easiest install wins" thesis already has a credible incumbent, and it
means nobody can verify its quality claims for you.

### Hezo

Teams of sandboxed agents with budget caps, own model keys, 12 UI languages,
CI and coverage badges, actively pushed. **GPL-3.0**, which is the important
part: AgentForge cannot borrow its code without becoming GPL.

### The two stalled single-binary projects

`hermes-agent-rust` (93 stars, last push 2026-07-08) and `Hector` (60 stars,
last push 2026-05-18) both made exactly the "one binary" pitch AgentForge
planned. Both have been quiet for months. One of them is a Rust rewrite of
Hermes, which is the same technical bet that worked for Goose and Open
Interpreter and appears not to have worked here.

This is a data point, not a proof. Two projects going quiet is not a trend, and
neither had meaningful adoption to begin with.

---

## The three proposed positions, re-examined

### 1. The easiest-to-install agent server

**Status: largely taken.** Gormes already ships a 55.6 MB binary across four
platforms including Android, with MIT and an offline doctor. Ollama set the
install bar years ago and has 181k stars of distribution behind it.

Surviving this position requires beating Gormes on a dimension other than
size, and beating Ollama on distribution. Neither is available to a project
with zero users.

### 2. One agent, many surfaces, one memory

**Status: this is where Hermes already is.** The Phase 1 brief listed Hermes
first, at 249k stars, multi-interface, actively developed. Building a smaller
version of that is not a position, it is a subset.

The genuine version of this idea is narrower: not "many surfaces" but "the
surfaces that do not exist yet" — and there is only one of those worth naming.
Nothing in the ten-plus projects reviewed is built mobile-first, where the
agent is reachable from a phone's own messaging apps rather than a browser.

### 3. Something for a phone

**Status: the strongest remaining position, and now the only real one.**

Gormes claims Android support in its download matrix. Hermes has a community
Termux tutorial and at least three blog posts about running it on a used phone.
So the *idea* is validated and being copied, which is the best possible sign.

But the Termux story in every case is a third-party guide. None of these
projects ship Termux as a first-class install target with its own build, its own
constraints documented, and a real test that it starts on an ARM phone.
Nobody is owning the phone.

---

## The honest verdict

**Position 3 is the project. Positions 1 and 2 are not.**

Stated as one sentence:

> AgentForge is the AI agent server that runs on a phone — installed and
> updated from the phone, tested on a phone, and reachable from a phone — and
> works the same on a VPS when the phone is not it.

That sentence is more specific than any of the six positioning attempts above,
and it is checkable: if it cannot be installed on a real Android device without
a community tutorial, the project has failed its own claim.

### What must NOT be in the MVP

- **A browser UI.** Open WebUI, LibreChat, and Flowise already saturate this,
  and a browser does not fit the phone positioning anyway.
- **A plugin marketplace.** A registry is an operating system problem. A
  project with ten users does not need one.
- **Multi-tenancy and auth.** There will be one user on one phone.
- **Memory as a database.** SQLite is the right answer and Gormes already uses
  it. Do not start with Postgres.
- **Docker as the primary install path.** Docker on Termux is a workaround, not
  a method.
- **A visual flow builder.** Langflow and Flowise own it and are excellent at it.

### The smallest useful version

A single binary that:

1. builds and runs on `aarch64-linux-android` under Termux
2. starts a local HTTP server bound to loopback
3. speaks to one model provider over HTTPS
4. answers on one interface, reachable from a phone — ideally the messaging app
   already on it
5. stores conversation state in one SQLite file the user can delete
6. has `doctor` output that says what is missing, offline, without spending tokens

Six things. If any one is missing on a real phone, the MVP is not done.

**Everything else — browser UI, plugin system, multi-agent, web dashboard — is
Phase 5 and later, and only if the six above work.**

---

## The check that matters most

Before writing any code, the following must be true:

> Someone must be able to run the build on a real Android phone, in Termux,
> without asking the author a question.

If the person building this does not have an Android phone, that is the first
thing to fix. Everything else can be simulated; Termux on a real ARM device
cannot, and the entire positioning rests on it.

---

## What was not decided

Phase 2 was supposed to produce an agreed MVP scope. The analysis above is a
recommendation, not a decision. Three things are still open and each one
changes the project:

1. **Is phone-first agreed as the position?** If the answer is no, this phase
   should be redone, because the other two positions are already occupied.
2. **Which interface first?** The original vision lists terminal, browser, API,
   and future mobile apps. Phone-first suggests a messaging app first, which is
   a different and harder build than a CLI.
3. **Is there an Android device to test on?** See the check above.
