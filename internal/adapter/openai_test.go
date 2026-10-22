package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cuteLittleDevil-ai/ai-diff-gate/internal/modelspec"
	"github.com/cuteLittleDevil-ai/ai-diff-gate/internal/provider"
)

func TestOpenAIChatStream(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body["stream"] != true {
			t.Errorf("expected stream=true, got %v", body["stream"])
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		for _, ev := range []string{
			`{"choices":[{"delta":{"content":"Hello"}}]}`,
			`{"choices":[{"delta":{"content":" world"}}]}`,
			`{"choices":[{"delta":{"content":"!"}}]}`,
		} {
			fmt.Fprintf(w, "data: %s\n\n", ev)
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer ts.Close()

	p := NewOpenAI(modelspec.Model{
		ID:        "openai-gpt",
		Model:     "gpt-4o-mini",
		BaseURL:   ts.URL,
		APIKeyEnv: "TEST_KEY",
	}, &http.Client{Timeout: 5 * time.Second})

	t.Setenv("TEST_KEY", "sk-test")

	var deltas []string
	onDelta := func(d provider.StreamDelta) {
		deltas = append(deltas, d.Content)
	}
	resp, err := p.ChatStream(context.Background(), &provider.ChatRequest{
		Messages: []provider.Message{{Role: "user", Content: "hi"}},
	}, onDelta)
	if err != nil {
		t.Fatalf("ChatStream failed: %v", err)
	}
	want := "Hello world!"
	if resp.Content != want {
		t.Errorf("content = %q, want %q", resp.Content, want)
	}
	got := strings.Join(deltas, "")
	if got != want {
		t.Errorf("deltas = %q, want %q", got, want)
	}
}

func TestOpenAIChatStreamError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, `{"error":{"message":"bad key"}}`)
	}))
	defer ts.Close()

	p := NewOpenAI(modelspec.Model{
		ID:        "openai-gpt",
		Model:     "gpt-4o-mini",
		BaseURL:   ts.URL,
		APIKeyEnv: "TEST_KEY",
	}, &http.Client{Timeout: 5 * time.Second})
	t.Setenv("TEST_KEY", "sk-test")

	_, err := p.ChatStream(context.Background(), &provider.ChatRequest{
		Messages: []provider.Message{{Role: "user", Content: "hi"}},
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "bad key") {
		t.Fatalf("expected error containing 'bad key', got %v", err)
	}
}
