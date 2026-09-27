package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/fsix7115-arch/AgentForge/internal/config"
	"github.com/fsix7115-arch/AgentForge/internal/store"
)

func TestEchoNeedsNoConfig(t *testing.T) {
	a, err := New(config.Config{Provider: "echo"})
	if err != nil {
		t.Fatalf("echo agent rejected: %v", err)
	}
	if !a.Offline() {
		t.Error("echo should report itself as offline")
	}
	reply, err := a.Reply(context.Background(), nil, "hello")
	if err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if !strings.Contains(reply, "hello") {
		t.Errorf("reply = %q, should echo the message", reply)
	}
}

// Building a provider without a key must fail at construction with a message
// naming the variable, not at request time with a 401.
func TestMissingKeyFailsAtConstruction(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("GROQ_API_KEY", "")

	for _, tc := range []struct{ provider, keyEnv string }{
		{"gemini", "GEMINI_API_KEY"},
		{"groq", "GROQ_API_KEY"},
	} {
		cfg := config.Config{Provider: tc.provider, Model: "m", APIKeyEnv: tc.keyEnv}
		_, err := New(cfg)
		if err == nil {
			t.Errorf("%s: expected an error with no key set", tc.provider)
			continue
		}
		if !strings.Contains(err.Error(), tc.keyEnv) {
			t.Errorf("%s: error %q should name %s", tc.provider, err, tc.keyEnv)
		}
	}
}

func TestUnknownProviderRejected(t *testing.T) {
	if _, err := New(config.Config{Provider: "nope"}); err == nil {
		t.Fatal("expected an error for an unknown provider")
	}
}

func TestHistoryTextKeepsRecencyAndTrimsOldest(t *testing.T) {
	// Build a long history so the trim path is exercised.
	var hist []store.Turn
	for i := 0; i < 400; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		hist = append(hist, store.Turn{Role: role, Content: strings.Repeat("x", 200)})
	}
	out := historyText(hist, "the newest question")

	if len(out) > 6100 {
		t.Errorf("output is %d chars, want it trimmed to about 6000", len(out))
	}
	if !strings.Contains(out, "the newest question") {
		t.Error("the latest question must survive trimming")
	}
	if !strings.Contains(out, "earlier turns trimmed") {
		t.Error("trimming should be visible, not silent")
	}
}

func TestHistoryTextLabelsRoles(t *testing.T) {
	out := historyText([]store.Turn{
		{Role: "user", Content: "q1"},
		{Role: "assistant", Content: "a1"},
	}, "q2")

	for _, want := range []string{"User: q1", "Assistant: a1", "User: q2"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\ngot:\n%s", want, out)
		}
	}
}

func TestEmptyHistoryStillIncludesTheMessage(t *testing.T) {
	out := historyText(nil, "just this")
	if strings.TrimSpace(out) != "User: just this" {
		t.Errorf("output = %q", out)
	}
}

// The Groq client must send a browser User-Agent, or Cloudflare rejects the
// request with error 1010 even when the key is valid. This is a real,
// reproducible failure, so it gets a test.
func TestBrowserUserAgentIsSet(t *testing.T) {
	ua := browserUA()
	if !strings.Contains(ua, "Mozilla/5.0") {
		t.Errorf("User-Agent = %q, want a browser UA to bypass Cloudflare", ua)
	}
}

func TestTruncateKeepsErrorsReadable(t *testing.T) {
	if got := truncate("short", 300); got != "short" {
		t.Errorf("truncate(short) = %q, want unchanged", got)
	}
	long := strings.Repeat("z", 500)
	got := truncate(long, 100)
	if len(got) != 103 {
		t.Errorf("truncate length = %d, want 103 (100 plus the ellipsis)", len(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Error("truncated output should end with an ellipsis")
	}
}

func TestEveryProviderHasAName(t *testing.T) {
	a, err := New(config.Config{Provider: "echo"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if a.Name() != "echo" {
		t.Errorf("Name() = %q, want echo", a.Name())
	}
}
