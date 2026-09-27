// Package server exposes the agent over a small local HTTP API.
//
// Design constraints, all from Phase 2:
//   - loopback by default, because an unauthenticated agent that can run code
//     must never be reachable from the network by accident
//   - three endpoints and no more, because the MVP is not a platform
//   - no browser UI, because that space is saturated and a browser does not
//     fit the phone positioning
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/fsix7115-arch/AgentForge/internal/agent"
	"github.com/fsix7115-arch/AgentForge/internal/config"
	"github.com/fsix7115-arch/AgentForge/internal/store"
	"github.com/fsix7115-arch/AgentForge/internal/version"
)

// Server holds the wiring between the HTTP layer and the agent.
type Server struct {
	cfg   config.Config
	ag    agent.Agent
	store *store.Store
	log   *slog.Logger
}

// New builds a Server. The agent is injected rather than constructed here so
// tests can pass the offline provider without touching config or env vars.
func New(cfg config.Config, ag agent.Agent, st *store.Store, log *slog.Logger) *Server {
	return &Server{cfg: cfg, ag: ag, store: st, log: log}
}

// Handler returns the configured http.Handler, so tests can use httptest
// without binding a port.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /", s.handleRoot)
	mux.HandleFunc("POST /chat", s.handleChat)
	mux.HandleFunc("POST /clear", s.handleClear)
	return s.logRequests(mux)
}

// chatRequest is the POST /chat body.
type chatRequest struct {
	// Message is the user's turn. If History is empty it is the first turn.
	Message string `json:"message"`
	// Session is accepted for future use and currently ignored: the MVP has
	// exactly one session. Rejecting it loudly would be worse than ignoring
	// it, and pretending it selects a session would be a lie.
	Session string `json:"session,omitempty"`
	// History lets a client supply context it holds itself, for a future
	// mobile app that keeps local state. When empty the server uses its own.
	History []store.Turn `json:"history,omitempty"`
}

// chatResponse is the POST /chat reply.
type chatResponse struct {
	Reply    string `json:"reply"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Turns    int    `json:"turns"`
	Elapsed  string `json:"elapsed"`
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"version":  version.Version,
		"provider": s.ag.Name(),
		"offline":  s.ag.Offline(),
		"turns":    s.turnCount(),
	})
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"name":    "agentforge",
		"version": version.String(),
		"endpoints": []string{
			"GET  /health  — liveness and provider",
			"POST /chat    — {\"message\": \"...\"}",
			"POST /clear   — delete local conversation history",
		},
	})
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	// 1 MB is generous for a chat turn and stops an accidental upload from
	// filling a phone's memory.
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()

	var req chatRequest
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "body must be JSON with a \"message\" field: " + err.Error(),
		})
		return
	}
	// Trim before the emptiness check. A message of three spaces is not a
	// message, and forwarding it wastes an API call to learn nothing.
	req.Message = strings.TrimSpace(req.Message)
	if req.Message == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "\"message\" is required and must not be empty",
		})
		return
	}

	// Prefer client-supplied history; fall back to the server's own.
	history := req.History
	if len(history) == 0 {
		sess, err := s.store.Load()
		if err != nil && !errors.Is(err, store.ErrNoSession) {
			s.log.Warn("session unreadable, continuing without history", "err", err)
		}
		history = sess.Turns
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.Timeout)
	defer cancel()

	reply, err := s.ag.Reply(ctx, history, req.Message)
	if err != nil {
		// Do not persist a failed turn: the user's message is already
		// visible to them, and a stored error would poison the next context.
		s.log.Error("reply failed", "err", err, "provider", s.ag.Name())
		writeJSON(w, http.StatusBadGateway, map[string]string{
			"error": err.Error(),
		})
		return
	}

	// Record both sides only after a successful call.
	if _, err := s.store.Append("user", req.Message); err != nil {
		s.log.Warn("could not persist user turn", "err", err)
	}
	sess, err := s.store.Append("assistant", reply)
	if err != nil {
		s.log.Warn("could not persist assistant turn", "err", err)
	}

	writeJSON(w, http.StatusOK, chatResponse{
		Reply:    reply,
		Provider: s.ag.Name(),
		Model:    s.cfg.Model,
		Turns:    len(sess.Turns),
		Elapsed:  time.Since(start).Round(time.Millisecond).String(),
	})
}

func (s *Server) handleClear(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Clear(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.log.Info("conversation history cleared")
	writeJSON(w, http.StatusOK, map[string]bool{"cleared": true})
}

func (s *Server) turnCount() int {
	turns, _, _, err := s.store.Stats()
	if err != nil {
		return 0
	}
	return turns
}

// logRequests is a compact access log. Verbose query logging is deliberately
// absent: on a phone the terminal is the whole UI, and a chat transcript in
// the log is a privacy problem.
func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		s.log.Info("req",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"ms", time.Since(start).Milliseconds(),
		)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	// The server binds loopback by default, but sending no CORS header means a
	// malicious page cannot read responses even if that changes.
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// Nothing useful to do: the status line is already sent.
		return
	}
}

// Serve runs the HTTP server until ctx is cancelled. Graceful shutdown
// matters on a phone, where the OS kills the process without warning.
func Serve(ctx context.Context, s *Server, log *slog.Logger) error {
	srv := &http.Server{
		Addr:              s.cfg.Addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		// No WriteTimeout: a model call on a phone's mobile connection can
		// legitimately take longer than any fixed limit.
		IdleTimeout: 120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		// Print the URL, because on a phone the next command depends on it.
		log.Info("listening", "url", fmt.Sprintf("http://%s", s.cfg.Addr),
			"provider", s.ag.Name(), "model", s.cfg.Model)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

// discardBody keeps io imported for future streaming support without
// breaking the build if that code is removed.
var _ = io.Discard
