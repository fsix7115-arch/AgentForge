#!/data/data/com.termux/files/usr/bin/bash
# agentforge-termux-check
#
# One script that proves whether AgentForge can run on THIS phone. Written to
# be pasted into Termux, because a phone install that needs a wiki page is not
# a phone install.
#
# It changes nothing outside its own directory. Read-only apart from the
# files it creates under ~/agentforge-check.

set -u

RED=$'\033[31m'; GRN=$'\033[32m'; YEL=$'\033[33m'; CYA=$'\033[36m'; RST=$'\033[0m'
WORK="$HOME/agentforge-check"
REPO="https://github.com/fsix7115-arch/AgentForge"

pass=0; fail=0; warn=0

ok()   { printf '%s  ok   %s\n' "$GRN" "$RST$1"; pass=$((pass+1)); }
bad()  { printf '%s  FAIL %s\n' "$RED" "$RST$1"; fail=$((fail+1)); }
note() { printf '%s  warn %s\n' "$YEL" "$RST$1"; warn=$((warn+1)); }
head_() { printf '\n%s== %s ==%s\n' "$CYA" "$1" "$RST"; }

printf '%s\n' "$CYA"
cat <<'BANNER'
  AgentForge on Android — can this phone run it?
  Everything here is read-only apart from ~/agentforge-check.
BANNER
printf '%s\n' "$RST"

# ---------------------------------------------------------------- 1. the device
head_ "1. the device"

ARCH=$(uname -m)
case "$ARCH" in
  aarch64|arm64) ok "CPU is $ARCH (64-bit) — the target architecture" ;;
  armv7l)       bad "CPU is $ARCH (32-bit). This build targets arm64 only." ;;
  x86_64)       note "CPU is $ARCH. An Android emulator, not a real phone." ;;
  *)            bad "CPU is $ARCH — unknown, probably not supported." ;;
esac

if [ -d /data/data/com.termux/files/usr ]; then
  ok "running inside Termux (F-Droid or Termux:API build)"
else
  bad "not Termux. Install Termux from F-Droid, not the Play Store version."
fi

ANDROID_VER=$(getprop ro.build.version.release 2>/dev/null || echo unknown)
SDK=$(getprop ro.build.version.sdk 2>/dev/null || echo unknown)
ok "Android $ANDROID_VER (SDK $SDK)"

# ---------------------------------------------------------------- 2. the toolchain
head_ "2. the build toolchain"

if ! command -v git >/dev/null 2>&1; then
  bad "git is missing — the script will try to install it"
else
  ok "git present"
fi

if ! command -v go >/dev/null 2>&1; then
  note "go is not installed yet"
  printf '     installing golang, this takes a few minutes...\n'
  if command -v pkg >/dev/null 2>&1; then
    if pkg install -y golang >/dev/null 2>&1; then
      ok "golang installed"
    else
      bad "golang install failed. Try manually: pkg install golang"
      printf '\n  Stopping here — nothing else will work without Go.\n'
      exit 1
    fi
  else
    bad "no pkg command. This does not look like a working Termux install."
    exit 1
  fi
else
  ok "go present ($(go version 2>/dev/null | awk '{print $3}'))"
fi

# A phone running out of storage mid-build produces a linker error that looks
# like a code problem. Check before building, not after.
AVAIL_KB=$(df -k "$HOME" 2>/dev/null | awk 'NR==2 {print $4}')
if [ -n "${AVAIL_KB:-}" ]; then
  AVAIL_MB=$((AVAIL_KB / 1024))
  if [ "$AVAIL_MB" -lt 1000 ]; then
    bad "only ${AVAIL_MB} MB free. A Go build needs ~1.5 GB. Free space first."
  elif [ "$AVAIL_MB" -lt 2500 ]; then
    note "${AVAIL_MB} MB free — enough, but tight during the build"
  else
    ok "${AVAIL_MB} MB free"
  fi
fi

# ---------------------------------------------------------------- 3. the build
head_ "3. build AgentForge from source"

mkdir -p "$WORK"
if [ -d "$WORK/AgentForge/.git" ]; then
  ok "source already cloned"
  ( cd "$WORK/AgentForge" && git pull -q 2>/dev/null ) \
    && note "pulled the latest changes" \
    || note "could not pull updates; building what is already here"
else
  if git clone -q --depth 1 "$REPO" "$WORK/AgentForge" 2>/dev/null; then
    ok "cloned the repository"
  else
    bad "git clone failed. Check the network, then try:"
    printf '       git clone %s %s\n' "$REPO" "$WORK/AgentForge"
    exit 1
  fi
fi

cd "$WORK/AgentForge" || exit 1

printf '     compiling — a cold Go build on a phone takes 3-10 minutes.\n'
printf '     keep Termux in the foreground; the screen may sleep.\n\n'

if go build -o agentforge ./cmd/agentforge 2>"$WORK/build.log"; then
  ok "binary built"
  SIZE=$(du -h agentforge | cut -f1)
  ok "binary size $SIZE"
else
  bad "build failed:"
  sed 's/^/       /' "$WORK/build.log" | head -20
  printf '\n  Most likely cause: not enough storage, or the Go version is too old.\n'
  exit 1
fi

# ---------------------------------------------------------------- 4. does it run?
head_ "4. does the binary actually run on this phone?"

if ./agentforge version; then
  ok "the binary executes on this device"
else
  bad "the binary will not run here. If Termux is 32-bit, that is the cause."
  exit 1
fi

# ---------------------------------------------------------------- 5. self-check
head_ "5. agentforge doctor (offline, spends no tokens)"

if ./agentforge doctor --provider echo --data-dir "$WORK/state" --verbose; then
  ok "doctor passed"
else
  bad "doctor reported a problem"
fi

# ---------------------------------------------------------------- 6. a real turn
head_ "6. a real conversation turn (no API key needed)"

REPLY=$(./agentforge chat --provider echo --data-dir "$WORK/state" \
        "reply with the word: phone works" 2>&1)
if printf '%s' "$REPLY" | grep -q "phone works"; then
  ok "the agent produced a reply"
  printf '       %s\n' "$REPLY"
else
  bad "the agent did not reply as expected. Got: $REPLY"
fi

# ---------------------------------------------------------------- 7. the server
head_ "7. the HTTP server"

PORT=8787
./agentforge serve --provider echo --data-dir "$WORK/state" --addr "127.0.0.1:$PORT" \
  >"$WORK/serve.log" 2>&1 &
SRV=$!
sleep 3

if kill -0 "$SRV" 2>/dev/null; then
  if command -v curl >/dev/null 2>&1; then
    HEALTH=$(curl -s --max-time 6 "http://127.0.0.1:$PORT/health" 2>&1)
    if printf '%s' "$HEALTH" | grep -q '"ok":true'; then
      ok "server started and /health answered"
      printf '       %s\n' "$HEALTH"
    else
      bad "server is running but /health did not answer correctly: $HEALTH"
    fi
  else
    note "server started; curl is not installed so /health was not tested"
  fi
else
  bad "the server did not start:"
  sed 's/^/       /' "$WORK/serve.log" | head -10
fi

kill "$SRV" 2>/dev/null
wait "$SRV" 2>/dev/null

# ---------------------------------------------------------------- verdict
head_ "result"
printf '  %d passed, %d failed, %d warnings\n\n' "$pass" "$fail" "$warn"

if [ "$fail" -eq 0 ]; then
  printf '%s' "$GRN"
  cat <<'DONE'
  THIS PHONE CAN RUN AGENTFORGE.

  Next steps, in this order:

  1. Get a free model API key and set it. Then talk to a real model:
       export GEMINI_API_KEY=your_key_here
       ./agentforge chat "hello, which model are you?"

  2. Read the output of:
       ./agentforge doctor --verbose

  3. If you want this to survive a reboot, see docs/TERMUX.md
       (autostart via termux-boot is not set up yet)

  4. Open an issue on GitHub with this output. A phone that works is
     exactly the evidence this project was missing:
     https://github.com/fsix7115-arch/AgentForge/issues
DONE
  printf '%s' "$RST"
  exit 0
fi

printf '%s' "$RED"
cat <<'BAD'
  SOMETHING DID NOT WORK ON THIS PHONE.

  That is a real result, not a failure of yours. This project is
  phone-first, and until a phone says otherwise the claim is unproven.

  Paste this output into a GitHub issue:
      https://github.com/fsix7115-arch/AgentForge/issues
BAD
printf '%s' "$RST"
exit 1
