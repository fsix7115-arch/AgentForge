# Running AgentForge on an Android phone (Termux)

**This is the page the project exists to justify.** The positioning is
phone-first. Until a real phone runs this, the claim is unproven — so the
first thing this document asks for is evidence, not enthusiasm.

Nothing here has been run on a real device yet. The build for `android/arm64`
compiles, which is necessary but not sufficient. **Please run the check script
and report the result, including if it fails.**

---

## The fast path

One command. In Termux, paste:

```bash
curl -fsSL https://raw.githubusercontent.com/fsix7115-arch/AgentForge/main/scripts/termux-check.sh | bash
```

That script will check the phone, install Go if missing, clone, build, and
verify the binary end to end. It prints a verdict at the end.

**Prefer not to pipe a script into a shell?** Download and read it first:

```bash
curl -fsSLO https://raw.githubusercontent.com/fsix7115-arch/AgentForge/main/scripts/termux-check.sh
less termux-check.sh
bash termux-check.sh
```

Piping a remote script into `bash` without reading it is a real risk. The
download-then-read form above is the one to use if you care about what you are
running.

---

## What you need first

**Termux, from F-Droid.** Not the Play Store version. The Play Store build
was deprecated and is stale — it lacks `pkg`, which means no Go, which means
no build.

1. Install [F-Droid](https://f-droid.org)
2. Add the [Termux repo](https://github.com/termux/termux-app/releases) — the
   F-Droid store still lists an old build
3. Install **Termux** and **Termux:API** if you want the API to be reachable
   from other apps later

A 32-bit phone will not work. Nearly every Android device made since 2017 is
`aarch64`, which is the target.

---

## The manual path

If you would rather do it step by step, or you want to understand each
command:

```bash
pkg update && pkg upgrade
pkg install git golang

git clone https://github.com/fsix7115-arch/AgentForge
cd AgentForge
go test ./...          # 51 tests, should say "ok" five times
go build -o agentforge ./cmd/agentforge
./agentforge doctor --verbose
```

A cold Go build on a phone takes **3-10 minutes**. That is normal. Keep
Termux in the foreground, and if the screen sleeps the process may be paused.

**Storage:** a Go build needs roughly 1.5 GB of free space, plus about 200 MB
for the source and the binary. Free space before you start. The check script
tests this first, because running out mid-build produces a linker error that
looks like a code problem and is not one.

---

## Using it

```bash
# no API key needed — a local provider that just echoes
./agentforge chat --provider echo "hello"

# with a real model
export GEMINI_API_KEY=your_key_here
./agentforge chat "which model are you?"

# the HTTP server, on loopback only
./agentforge serve
```

From another app on the same phone, with Termux:API installed:

```bash
curl -s localhost:8787/chat -d '{"message":"hi"}'
```

### Keeping the key

Do not put the key in the command history. Create `~/.bashrc`:

```bash
echo 'export GEMINI_API_KEY=your_key_here' >> ~/.bashrc
```

That file is inside Termux's private data directory, which no other app can
read without root. It is still a plain text file, so do not share it.

---

## Surviving a reboot

**Not implemented yet.** The usual approach on Termux is:

1. Install [Termux:Boot](https://f-droid.org/packages/com.termux.boot/) from
   F-Droid
2. Create `~/.termux/boot/start-agentforge`:

```bash
mkdir -p ~/.termux/boot
cat > ~/.termux/boot/start-agentforge <<'EOF'
#!/data/data/com.termux/files/usr/bin/bash
termux-wake-lock
cd ~/agentforge-check/AgentForge
./agentforge serve >> ~/agentforge.log 2>&1
EOF
chmod +x ~/.termux/boot/start-agentforge
```

`termux-wake-lock` matters: without it Android will kill the process
whenever it feels like it. This is a known rough edge of running anything
long-lived on a phone, and it is listed as not-done in the main README rather
than hidden here.

---

## Troubleshooting

**`pkg: command not found`** — the Play Store build of Termux. Uninstall it
and install the F-Droid one. There is no fix on the same install.

**Build fails with `no space left on device`** — free storage. A Go build
wants ~1.5 GB. `pkg clean` helps.

**Build fails with `requires external (cgo) linking`** — you are trying to
build `android/amd64`. That is not a target; use arm64. Check with `uname -m`.

**Binary built but `./agentforge: cannot execute`** — usually a 32-bit
Termux. Run `uname -m`; if it says `armv7l`, install the 64-bit Termux build
or use a different device.

**`doctor` says the port is already in use** — another instance is running.
`pkill -f agentforge` and start again.

**`401 Invalid API Key` from Groq or Gemini** — the key is in the wrong
variable. `agentforge doctor --verbose` prints which variable it is reading
and how many characters it found, never the value. Groq reads `GROQ_API_KEY`;
Gemini reads `GEMINI_API_KEY`.

---

## Reporting

Whether it works or fails, the output matters. Open an issue:

**https://github.com/fsix7115-arch/AgentForge/issues**

A phone that runs it is the single piece of evidence this project lacks. A
phone that fails in a specific way is nearly as valuable, because it turns
"unproven" into "known broken on device X with error Y".
