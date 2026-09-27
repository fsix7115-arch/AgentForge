package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func newTestStore(t *testing.T, limit int) (*Store, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "session", "agentforge.db")
	return New(path, limit), path
}

func TestLoadBeforeAnyTurnIsNotAnError(t *testing.T) {
	s, _ := newTestStore(t, 10)
	_, err := s.Load()
	if !errors.Is(err, ErrNoSession) {
		t.Fatalf("Load() error = %v, want ErrNoSession", err)
	}
}

func TestAppendCreatesAndPersists(t *testing.T) {
	s, path := newTestStore(t, 10)

	if _, err := s.Append("user", "hello"); err != nil {
		t.Fatalf("append user: %v", err)
	}
	sess, err := s.Append("assistant", "hi there")
	if err != nil {
		t.Fatalf("append assistant: %v", err)
	}
	if len(sess.Turns) != 2 {
		t.Fatalf("got %d turns, want 2", len(sess.Turns))
	}

	// The file must exist, be private, and be readable with cat — a phone
	// user should never need a tool to inspect their own history.
	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := st.Mode().Perm(); perm != 0o600 {
		t.Errorf("file mode = %o, want 600 — history is private", perm)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var decoded Session
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("the on-disk file is not valid JSON: %v", err)
	}
	if decoded.Turns[0].Content != "hello" {
		t.Errorf("first turn = %q, want hello", decoded.Turns[0].Content)
	}
}

// A phone with limited storage must not accumulate history forever. Oldest
// turns are dropped, newest kept.
func TestTurnLimitTrimsOldest(t *testing.T) {
	s, _ := newTestStore(t, 3)

	for _, msg := range []string{"one", "two", "three", "four", "five"} {
		if _, err := s.Append("user", msg); err != nil {
			t.Fatalf("append %q: %v", msg, err)
		}
	}

	sess, err := s.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(sess.Turns) != 3 {
		t.Fatalf("got %d turns, want 3", len(sess.Turns))
	}
	if sess.Turns[0].Content != "three" {
		t.Errorf("oldest kept = %q, want three", sess.Turns[0].Content)
	}
	if sess.Turns[2].Content != "five" {
		t.Errorf("newest kept = %q, want five", sess.Turns[2].Content)
	}
}

// A phone can be killed mid-write. A partial file would make the history
// permanently unreadable, so the write must be atomic.
func TestWriteIsAtomic(t *testing.T) {
	s, path := newTestStore(t, 10)
	if _, err := s.Append("user", "first"); err != nil {
		t.Fatalf("append: %v", err)
	}

	// No .tmp file may survive a successful write.
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Error("a .tmp file was left behind after a successful save")
	}

	// And the file must parse at every point, not just at the end.
	for i := 0; i < 20; i++ {
		if _, err := s.Append("user", "message"); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read after %d appends: %v", i, err)
		}
		var probe Session
		if err := json.Unmarshal(b, &probe); err != nil {
			t.Fatalf("file unparseable after %d appends: %v", i, err)
		}
	}
}

func TestClearRemovesHistory(t *testing.T) {
	s, path := newTestStore(t, 10)
	if _, err := s.Append("user", "secret"); err != nil {
		t.Fatalf("append: %v", err)
	}
	if err := s.Clear(); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("Clear did not remove the file")
	}
	// Clearing twice must not error — a phone user will do it twice.
	if err := s.Clear(); err != nil {
		t.Errorf("second Clear errored: %v", err)
	}
}

func TestStatsOnMissingFile(t *testing.T) {
	s, _ := newTestStore(t, 10)
	turns, size, exists, err := s.Stats()
	if err != nil {
		t.Fatalf("Stats on a missing file errored: %v", err)
	}
	if exists || turns != 0 || size != 0 {
		t.Errorf("Stats = (%d, %d, %v), want zeros and false", turns, size, exists)
	}
}

func TestCorruptFileReportsErrorRatherThanEmpty(t *testing.T) {
	s, path := newTestStore(t, 10)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Silently returning an empty session would look like data loss to the
	// user. It must be reported.
	_, err := s.Load()
	if err == nil {
		t.Fatal("expected a parse error for a corrupt session file")
	}
	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("error = %q, want it to mention parsing", err)
	}
}

// Two requests arriving at once must not interleave writes. This is the
// failure mode that actually shows up once a browser UI or messaging
// integration is added.
func TestConcurrentAppendsAreSerialised(t *testing.T) {
	s, _ := newTestStore(t, 1000)

	var wg sync.WaitGroup
	errs := make(chan error, 50)
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			if _, err := s.Append("user", "concurrent"); err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent append failed: %v", err)
	}

	sess, err := s.Load()
	if err != nil {
		t.Fatalf("load after concurrency: %v", err)
	}
	if len(sess.Turns) != 50 {
		t.Errorf("got %d turns, want 50 — a write was lost", len(sess.Turns))
	}
}

func TestTimestampsAreUTC(t *testing.T) {
	s, _ := newTestStore(t, 10)
	sess, err := s.Append("user", "hi")
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	if sess.Turns[0].Timestamp.Location() != time.UTC {
		t.Error("timestamps must be stored in UTC, not local time")
	}
}
