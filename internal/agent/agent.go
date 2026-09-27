// Package agent turns a message plus history into a reply.
//
// The interface is deliberately tiny — one method — because the MVP supports
// three providers and adding a fourth should not require touching the server
// or the CLI.
package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/fsix7115-arch/AgentForge/internal/config"
	"github.com/fsix7115-arch/AgentForge/internal/store"
)

// Agent produces replies.
type Agent interface {
	// Name identifies the backend in logs and `doctor` output.
	Name() string
	// Reply returns the assistant's answer to the latest turn, given the
	// prior conversation for context.
	Reply(ctx context.Context, history []store.Turn, userMsg string) (string, error)
	// Offline reports whether this backend can answer without a network call
	// and without an API key. Only the echo provider qualifies.
	Offline() bool
}

// New builds the agent named by cfg.Provider.
func New(cfg config.Config) (Agent, error) {
	switch cfg.Provider {
	case "echo":
		return &echoAgent{}, nil
	case "gemini":
		if cfg.APIKey() == "" {
			return nil, fmt.Errorf("provider gemini needs %s to be set", cfg.APIKeyEnv)
		}
		return &geminiAgent{cfg: cfg, http: http.DefaultClient}, nil
	case "groq":
		if cfg.APIKey() == "" {
			return nil, fmt.Errorf("provider groq needs %s to be set", cfg.APIKeyEnv)
		}
		return &groqAgent{cfg: cfg, http: http.DefaultClient}, nil
	default:
		// config.Validate already rejects this; belt and braces.
		return nil, fmt.Errorf("unknown provider %q", cfg.Provider)
	}
}

// historyText flattens the transcript for providers that take a single prompt
// rather than a structured message list. Truncates from the oldest side so a
// long conversation still fits the model's context.
func historyText(history []store.Turn, userMsg string) string {
	var b strings.Builder
	const maxChars = 6000
	for _, t := range history {
		prefix := "User: "
		if t.Role == "assistant" {
			prefix = "Assistant: "
		}
		b.WriteString(prefix + t.Content + "\n")
	}
	b.WriteString("User: " + userMsg)
	s := b.String()
	if len(s) > maxChars {
		// Keep the tail: recent context matters more than old.
		s = "...[earlier turns trimmed]...\n" + s[len(s)-maxChars:]
	}
	return s
}

// ---------------------------------------------------------------------------
// echo — the offline provider
// ---------------------------------------------------------------------------

// echoAgent answers without a network call. It exists so that `doctor` and
// the test suite work on a plane, on a phone with no data, and in CI.
type echoAgent struct{}

func (a *echoAgent) Name() string  { return "echo" }
func (a *echoAgent) Offline() bool { return true }

func (a *echoAgent) Reply(ctx context.Context, history []store.Turn, userMsg string) (string, error) {
	n := len(history) + 1
	return fmt.Sprintf("echo[%d]: %s (offline provider, no model called)", n, userMsg), nil
}

// ---------------------------------------------------------------------------
// gemini
// ---------------------------------------------------------------------------

type geminiAgent struct {
	cfg  config.Config
	http *http.Client
}

func (a *geminiAgent) Name() string  { return "gemini" }
func (a *geminiAgent) Offline() bool { return false }

func (a *geminiAgent) Reply(ctx context.Context, history []store.Turn, userMsg string) (string, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent",
		a.cfg.Model)

	payload := map[string]any{
		"contents": []map[string]any{{
			"parts": []map[string]string{{"text": historyText(history, userMsg)}},
		}},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", a.cfg.APIKey())

	resp, err := a.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("gemini request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("gemini read: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gemini %d: %s", resp.StatusCode, truncate(string(raw), 300))
	}

	var parsed struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("gemini parse: %w", err)
	}
	if len(parsed.Candidates) == 0 || len(parsed.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("gemini returned no content")
	}
	return parsed.Candidates[0].Content.Parts[0].Text, nil
}

// ---------------------------------------------------------------------------
// groq
// ---------------------------------------------------------------------------

type groqAgent struct {
	cfg  config.Config
	http *http.Client
}

func (a *groqAgent) Name() string  { return "groq" }
func (a *groqAgent) Offline() bool { return false }

// browserUA works around Cloudflare's bot rule on the Groq API. A bare Go
// user-agent gets error 1010 from Cloud Shell even though the key is valid.
func browserUA() string {
	return "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36"
}

func (a *groqAgent) Reply(ctx context.Context, history []store.Turn, userMsg string) (string, error) {
	payload := map[string]any{
		"model": a.cfg.Model,
		"messages": []map[string]string{
			{"role": "system", "content": "You are AgentForge, a helpful assistant running on the user's own device."},
			{"role": "user", "content": historyText(history, userMsg)},
		},
		"temperature": 0.7,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.groq.com/openai/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.cfg.APIKey())
	req.Header.Set("User-Agent", browserUA())

	resp, err := a.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("groq request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("groq read: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("groq %d: %s", resp.StatusCode, truncate(string(raw), 300))
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("groq parse: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("groq returned no choices")
	}
	return parsed.Choices[0].Message.Content, nil
}

// truncate keeps error messages readable in a terminal.
func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
