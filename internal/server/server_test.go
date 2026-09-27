package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fsix7115-arch/AgentForge/internal/config"
	"github.com/fsix7115-arch/AgentForge/internal/store"
)

// stubAgent returns a fixed reply and records what history it was given, so
// tests can assert on context handling without any network.
type stubAgent struct {
	reply      string
	err        error
	gotHistory []store.Turn
	gotMessage string
}

func (s *stubAgent) Name() string  { return "stub" }
func (s *stubAgent) Offline() bool { return true }

func (s *stubAgent) Reply(ctx context.Context, history []store.Turn, msg string) (string, error) {
	s.gotHistory = history
	s.gotMessage = msg
	return s.reply, s.err
}

func newTestServer(t *testing.T, ag *stubAgent) (*httptest.Server, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	st := store.New(dir+"/agentforge.db", 20)

	cfg := config.Defaults()
	cfg.Provider = "echo"

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := httptest.NewServer(New(cfg, ag, st, log).Handler())
	t.Cleanup(srv.Close)
	return srv, st
}

func postJSON(t *testing.T, url, body string) (int, map[string]any) {
	t.Helper()
	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer resp.Body.Close()

	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp.StatusCode, out
}

func TestHealthReportsOK(t *testing.T) {
	ag := &stubAgent{reply: "ok"}
	srv, _ := newTestServer(t, ag)

	resp, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out["ok"] != true {
		t.Errorf("ok = %v, want true", out["ok"])
	}
	if out["provider"] != "stub" {
		t.Errorf("provider = %v, want stub", out["provider"])
	}
}

func TestChatReturnsReplyAndPersists(t *testing.T) {
	ag := &stubAgent{reply: "hello from agent"}
	srv, st := newTestServer(t, ag)

	status, out := postJSON(t, srv.URL+"/chat", `{"message":"hi there"}`)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if out["reply"] != "hello from agent" {
		t.Errorf("reply = %v", out["reply"])
	}
	if out["provider"] != "stub" {
		t.Errorf("provider = %v", out["provider"])
	}

	// Both sides of the exchange must be on disk.
	sess, err := st.Load()
	if err != nil {
		t.Fatalf("load session: %v", err)
	}
	if len(sess.Turns) != 2 {
		t.Fatalf("got %d turns, want 2", len(sess.Turns))
	}
	if sess.Turns[0].Content != "hi there" || sess.Turns[1].Content != "hello from agent" {
		t.Errorf("stored turns = %+v", sess.Turns)
	}
}

func TestEmptyMessageRejected(t *testing.T) {
	ag := &stubAgent{reply: "should not be called"}
	srv, _ := newTestServer(t, ag)

	for _, body := range []string{`{"message":""}`, `{}`, `{"message":"   "}`} {
		status, out := postJSON(t, srv.URL+"/chat", body)
		if status != http.StatusBadRequest {
			t.Errorf("body %s: status = %d, want 400", body, status)
		}
		if _, hasErr := out["error"]; !hasErr {
			t.Errorf("body %s: response has no error field: %v", body, out)
		}
	}
	if ag.gotMessage != "" {
		t.Error("the agent was called for an invalid request")
	}
}

func TestMalformedJSONRejected(t *testing.T) {
	ag := &stubAgent{reply: "x"}
	srv, _ := newTestServer(t, ag)

	status, _ := postJSON(t, srv.URL+"/chat", `{"message": `)
	if status != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", status)
	}
}

func TestHistoryIsPassedToTheAgent(t *testing.T) {
	ag := &stubAgent{reply: "second answer"}
	srv, _ := newTestServer(t, ag)

	postJSON(t, srv.URL+"/chat", `{"message":"first question"}`)
	postJSON(t, srv.URL+"/chat", `{"message":"second question"}`)

	if len(ag.gotHistory) == 0 {
		t.Fatal("the agent received no prior history on the second turn")
	}
	if ag.gotMessage != "second question" {
		t.Errorf("latest message = %q", ag.gotMessage)
	}
}

// A client that keeps its own history (a future mobile app) must be able to
// supply it rather than relying on server state.
func TestClientSuppliedHistoryWins(t *testing.T) {
	ag := &stubAgent{reply: "ok"}
	srv, _ := newTestServer(t, ag)

	body := `{"message":"q","history":[
		{"role":"user","content":"client turn 1","ts":"2026-01-01T00:00:00Z"},
		{"role":"assistant","content":"client turn 2","ts":"2026-01-01T00:00:01Z"}]}`
	status, _ := postJSON(t, srv.URL+"/chat", body)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if len(ag.gotHistory) != 2 {
		t.Fatalf("agent got %d history turns, want the 2 supplied", len(ag.gotHistory))
	}
	if ag.gotHistory[0].Content != "client turn 1" {
		t.Errorf("history[0] = %q, want the client's own turn", ag.gotHistory[0].Content)
	}
}

// A provider failure must not be persisted: storing an error would poison the
// context of every later turn.
func TestProviderErrorIsNotPersisted(t *testing.T) {
	ag := &stubAgent{err: errors.New("provider exploded")}
	srv, st := newTestServer(t, ag)

	status, out := postJSON(t, srv.URL+"/chat", `{"message":"doomed"}`)
	if status != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", status)
	}
	if msg, _ := out["error"].(string); !strings.Contains(msg, "provider exploded") {
		t.Errorf("error = %q, want the provider message passed through", msg)
	}

	_, size, exists, _ := st.Stats()
	if exists && size > 0 {
		t.Error("a failed turn was written to disk")
	}
}

func TestOversizedBodyRejected(t *testing.T) {
	ag := &stubAgent{reply: "x"}
	srv, _ := newTestServer(t, ag)

	big := `{"message":"` + strings.Repeat("A", 2<<20) + `"}`
	status, _ := postJSON(t, srv.URL+"/chat", big)
	if status == http.StatusOK {
		t.Error("a 2 MB body was accepted; the 1 MB limit is not enforced")
	}
}

func TestClearEmptiesHistory(t *testing.T) {
	ag := &stubAgent{reply: "answer"}
	srv, st := newTestServer(t, ag)

	postJSON(t, srv.URL+"/chat", `{"message":"remember this"}`)

	status, out := postJSON(t, srv.URL+"/clear", `{}`)
	if status != http.StatusOK {
		t.Fatalf("clear status = %d, want 200", status)
	}
	if out["cleared"] != true {
		t.Errorf("cleared = %v, want true", out["cleared"])
	}

	if _, _, exists, _ := st.Stats(); exists {
		t.Error("history still exists after clear")
	}
}

func TestUnknownPathIs404(t *testing.T) {
	ag := &stubAgent{reply: "x"}
	srv, _ := newTestServer(t, ag)

	resp, err := http.Get(srv.URL + "/nope")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestSecurityHeadersPresent(t *testing.T) {
	ag := &stubAgent{reply: "x"}
	srv, _ := newTestServer(t, ag)

	resp, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
	// No CORS header means a hostile page cannot read responses even if the
	// bind address is later changed.
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want none", got)
	}
}

// Echoing an arbitrary message back is fine, but a reply containing HTML
// must not be served as HTML.
func TestContentTypeIsJSON(t *testing.T) {
	ag := &stubAgent{reply: "<script>alert(1)</script>"}
	srv, _ := newTestServer(t, ag)

	resp, err := http.Post(srv.URL+"/chat", "application/json", strings.NewReader(`{"message":"x"}`))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want JSON", ct)
	}
}
