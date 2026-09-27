// Package store persists conversation state.
//
// The MVP deliberately does not use SQLite. Gormes uses SQLite, which is the
// right long-term answer, but a phone-first project pays a cgo or a pure-Go
// driver cost for it, and the whole point of the MVP is that it runs on a
// device with no build toolchain. State here is a JSON file: inspectable with
// cat, deletable with rm, and recoverable by hand if it ever corrupts.
//
// This is expected to be replaced. The interface below is the seam.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Turn is one exchange: what was asked and what was answered.
type Turn struct {
	Role      string    `json:"role"` // "user" or "assistant"
	Content   string    `json:"content"`
	Timestamp time.Time `json:"ts"`
}

// Session is a named conversation. The MVP has exactly one.
type Session struct {
	ID        string    `json:"id"`
	Created   time.Time `json:"created"`
	Updated   time.Time `json:"updated"`
	Turns     []Turn    `json:"turns"`
	TurnLimit int       `json:"turn_limit"`
}

// Store is the persistence surface. Keep it small so the SQLite replacement
// only has to satisfy these methods.
type Store struct {
	path      string
	turnLimit int
	mu        sync.Mutex
}

// New opens (or will create) a store at path.
func New(path string, turnLimit int) *Store {
	if turnLimit <= 0 {
		turnLimit = 40
	}
	return &Store{path: path, turnLimit: turnLimit}
}

// Path is the backing file, exposed for `doctor`.
func (s *Store) Path() string { return s.path }

// ErrNoSession means the session file does not exist yet. It is not fatal:
// the first turn creates it.
var ErrNoSession = errors.New("no session yet")

// Load reads the session, or returns ErrNoSession if there is none.
func (s *Store) Load() (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return Session{}, ErrNoSession
	}
	if err != nil {
		return Session{}, fmt.Errorf("read %s: %w", s.path, err)
	}
	var sess Session
	if err := json.Unmarshal(b, &sess); err != nil {
		return Session{}, fmt.Errorf("parse %s: %w", s.path, err)
	}
	return sess, nil
}

// Append adds a turn and persists. The write is atomic: a temp file in the
// same directory, then rename. A phone that dies mid-write gets the old file,
// never a half-written one.
func (s *Store) Append(role, content string) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	sess, err := s.loadLocked()
	if err != nil && !errors.Is(err, ErrNoSession) {
		return Session{}, err
	}
	if errors.Is(err, ErrNoSession) {
		sess = Session{
			ID:        "default",
			Created:   now,
			TurnLimit: s.turnLimit,
		}
	}
	if sess.TurnLimit <= 0 {
		sess.TurnLimit = s.turnLimit
	}

	sess.Turns = append(sess.Turns, Turn{Role: role, Content: content, Timestamp: now})
	// Trim oldest first. A phone has little storage and an unbounded
	// transcript is how you fill it.
	if len(sess.Turns) > sess.TurnLimit {
		sess.Turns = sess.Turns[len(sess.Turns)-sess.TurnLimit:]
	}
	sess.Updated = now

	if err := s.saveLocked(sess); err != nil {
		return Session{}, err
	}
	return sess, nil
}

// Clear deletes the session file. There is no soft delete — a phone user
// should be able to remove their history with one command.
func (s *Store) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("clear %s: %w", s.path, err)
	}
	return nil
}

// Stats is what `doctor` reports about local state.
func (s *Store) Stats() (turns int, sizeBytes int64, created bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	st, statErr := os.Stat(s.path)
	if errors.Is(statErr, os.ErrNotExist) {
		return 0, 0, false, nil
	}
	if statErr != nil {
		return 0, 0, false, statErr
	}
	sess, loadErr := s.loadLocked()
	if loadErr != nil {
		return 0, st.Size(), true, loadErr
	}
	return len(sess.Turns), st.Size(), true, nil
}

func (s *Store) loadLocked() (Session, error) {
	b, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return Session{}, ErrNoSession
	}
	if err != nil {
		return Session{}, err
	}
	var sess Session
	if err := json.Unmarshal(b, &sess); err != nil {
		return Session{}, err
	}
	return sess, nil
}

func (s *Store) saveLocked(sess Session) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	b, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return fmt.Errorf("encode session: %w", err)
	}
	// 0600: conversation history is private, and on a shared or rooted phone
	// that matters.
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return fmt.Errorf("write temp: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("commit session: %w", err)
	}
	return nil
}
