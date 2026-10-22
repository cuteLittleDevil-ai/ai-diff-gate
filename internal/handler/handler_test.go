package handler

import (
	"context"
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cuteLittleDevil-ai/ai-diff-gate/internal/config"
	"github.com/cuteLittleDevil-ai/ai-diff-gate/internal/provider"
)

func TestHandleCompareStream(t *testing.T) {
	cfg := &config.Config{
		Server:  config.ServerConfig{Addr: ":8080"},
		Fanout:  config.FanoutConfig{TimeoutSeconds: 5},
		Compare: config.CompareConfig{Granularity: "line"},
		Models: []config.ModelConfig{
			{ID: "model-a", Provider: "mock", DisplayName: "Model A", Enabled: boolPtr(true)},
			{ID: "model-b", Provider: "mock", DisplayName: "Model B", Enabled: boolPtr(true)},
		},
	}

	providers := []provider.Provider{
		&provider.MockProvider{IDVal: "model-a", DisplayName_: "Model A", Deltas: []string{"alpha", " beta"}},
		&provider.MockProvider{IDVal: "model-b", DisplayName_: "Model B", Deltas: []string{"alpha", " gamma"}},
	}

	tmpl := template.Must(template.New("compare.html").Parse("<html></html>"))
	srv := New(cfg, providers, tmpl)
	mux := http.NewServeMux()
	srv.Register(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/compare/stream", strings.NewReader(`{"prompt":"x","model_ids":["model-a","model-b"]}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/event-stream") {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}

	body := w.Body.String()
	if !strings.Contains(body, "event: model_start") {
		t.Errorf("missing model_start event; body:\n%s", body)
	}
	if !strings.Contains(body, `event: delta`) {
		t.Errorf("missing delta event; body:\n%s", body)
	}
	if !strings.Contains(body, `event: model_done`) {
		t.Errorf("missing model_done event; body:\n%s", body)
	}
	if !strings.Contains(body, `event: diff`) {
		t.Errorf("missing diff event; body:\n%s", body)
	}
	if !strings.Contains(body, `event: done`) {
		t.Errorf("missing done event; body:\n%s", body)
	}
	if !strings.Contains(body, "alpha beta") || !strings.Contains(body, "alpha gamma") {
		t.Errorf("expected streamed content in body; body:\n%s", body)
	}
}

func TestHandleCompareStreamMissingPrompt(t *testing.T) {
	cfg := &config.Config{}
	tmpl := template.Must(template.New("compare.html").Parse("<html></html>"))
	srv := New(cfg, nil, tmpl)
	mux := http.NewServeMux()
	srv.Register(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/compare/stream", strings.NewReader(`{"model_ids":[]}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandleCompareJSONStillWorks(t *testing.T) {
	cfg := &config.Config{
		Server:  config.ServerConfig{Addr: ":8080"},
		Fanout:  config.FanoutConfig{TimeoutSeconds: 5},
		Compare: config.CompareConfig{Granularity: "line"},
		Models: []config.ModelConfig{
			{ID: "model-a", Provider: "mock", DisplayName: "Model A", Enabled: boolPtr(true)},
		},
	}

	providers := []provider.Provider{
		&provider.MockProvider{IDVal: "model-a", DisplayName_: "Model A", Resp: &provider.ChatResponse{Content: "hello", Model: "model-a"}},
	}

	tmpl := template.Must(template.New("compare.html").Parse("<html></html>"))
	srv := New(cfg, providers, tmpl)
	mux := http.NewServeMux()
	srv.Register(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/compare", strings.NewReader(`{"prompt":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"content":"hello"`) {
		t.Errorf("expected non-streaming JSON response; body: %s", w.Body.String())
	}
}

func boolPtr(b bool) *bool { return &b }

// satisfy context import lint guard in other test files if needed
var _ = context.Background
var _ = io.ReadAll
