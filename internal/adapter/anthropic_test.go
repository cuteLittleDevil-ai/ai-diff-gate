package adapter

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cuteLittleDevil-ai/ai-diff-gate/internal/modelspec"
	"github.com/cuteLittleDevil-ai/ai-diff-gate/internal/provider"
)

func TestAnthropicChatStream(t *testing.T) {
	streamBody := `event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hi"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" there"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"input_tokens":5,"output_tokens":2}}

event: message_stop
data: {"type":"message_stop"}

`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
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
		io.WriteString(w, streamBody)
	}))
	defer ts.Close()

	p := NewAnthropic(modelspec.Model{
		ID:        "anthropic-claude",
		Model:     "claude-3-5-sonnet-latest",
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
	want := "Hi there"
	if resp.Content != want {
		t.Errorf("content = %q, want %q", resp.Content, want)
	}
	got := strings.Join(deltas, "")
	if got != want {
		t.Errorf("deltas = %q, want %q", got, want)
	}
	if resp.TotalTokens == nil || *resp.TotalTokens != 7 {
		t.Errorf("total tokens = %v, want 7", resp.TotalTokens)
	}
}
