package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/cuteLittleDevil-ai/ai-diff-gate/internal/compare"
	"github.com/cuteLittleDevil-ai/ai-diff-gate/internal/config"
	"github.com/cuteLittleDevil-ai/ai-diff-gate/internal/fanout"
	"github.com/cuteLittleDevil-ai/ai-diff-gate/internal/provider"
)

// Server handles HTTP routes for compare UI and JSON API.
type Server struct {
	cfg       *config.Config
	providers map[string]provider.Provider
	runner    *fanout.Runner
	tmpl      *template.Template
}

// New creates an HTTP handler server.
func New(cfg *config.Config, providers []provider.Provider, tmpl *template.Template) *Server {
	byID := make(map[string]provider.Provider, len(providers))
	for _, p := range providers {
		byID[p.ID()] = p
	}
	return &Server{
		cfg:       cfg,
		providers: byID,
		runner:    &fanout.Runner{Timeout: cfg.FanoutTimeout()},
		tmpl:      tmpl,
	}
}

// Register mounts routes on mux.
func (s *Server) Register(mux *http.ServeMux) {
	mux.HandleFunc("/", s.handleComparePage)
	mux.HandleFunc("/compare", s.handleComparePage)
	mux.HandleFunc("/api/v1/compare", s.handleCompareJSON)
	mux.HandleFunc("/api/v1/compare/stream", s.handleCompareStream)
}

func (s *Server) handleComparePage(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.renderPage(w, pageData{
			Models: s.listModelOptions(),
		})
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		prompt := strings.TrimSpace(r.FormValue("prompt"))
		if prompt == "" {
			http.Error(w, "prompt is required", http.StatusBadRequest)
			return
		}
		modelIDs := r.Form["models"]
		providers, err := s.resolveProviders(modelIDs)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		results := s.runner.Run(r.Context(), prompt, providers)
		diffs := compare.BuildDiffs(s.cfg.Compare, results)
		s.renderPage(w, pageData{
			Prompt:   prompt,
			Models:   s.listModelOptions(),
			Selected: modelIDs,
			Results:  results,
			Diffs:    diffs,
		})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

type compareJSONRequest struct {
	Prompt   string   `json:"prompt"`
	ModelIDs []string `json:"model_ids"`
}

type compareJSONResponse struct {
	Prompt  string           `json:"prompt"`
	Results []jsonResult     `json:"results"`
	Diffs   []compare.PairDiff `json:"diffs"`
}

type jsonResult struct {
	ModelID          string `json:"model_id"`
	DisplayName      string `json:"display_name"`
	OK               bool   `json:"ok"`
	Content          string `json:"content,omitempty"`
	Error            string `json:"error,omitempty"`
	LatencyMs        int64  `json:"latency_ms"`
	PromptTokens     *int   `json:"prompt_tokens,omitempty"`
	CompletionTokens *int   `json:"completion_tokens,omitempty"`
	TotalTokens      *int   `json:"total_tokens,omitempty"`
}

func (s *Server) handleCompareJSON(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}
	var req compareJSONRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		http.Error(w, "prompt is required", http.StatusBadRequest)
		return
	}
	providers, err := s.resolveProviders(req.ModelIDs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	results := s.runner.Run(r.Context(), prompt, providers)
	diffs := compare.BuildDiffs(s.cfg.Compare, results)

	resp := compareJSONResponse{
		Prompt: prompt,
		Diffs:  diffs,
	}
	for _, res := range results {
		jr := jsonResult{
			ModelID:          res.ModelID,
			DisplayName:      res.DisplayName,
			OK:               res.OK,
			LatencyMs:        res.LatencyMs,
			PromptTokens:     res.PromptTokens,
			CompletionTokens: res.CompletionTokens,
			TotalTokens:      res.TotalTokens,
		}
		if res.OK {
			jr.Content = res.Content
		} else {
			jr.Error = res.Error
		}
		resp.Results = append(resp.Results, jr)
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("encode json response: %v", err)
	}
}

func (s *Server) handleCompareStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}
	var req compareJSONRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		http.Error(w, "prompt is required", http.StatusBadRequest)
		return
	}
	providers, err := s.resolveProviders(req.ModelIDs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flush(w)

	out := make(chan fanout.StreamEvent, 32)
	go s.runner.RunStream(r.Context(), prompt, providers, out)

	var results []fanout.ModelResult
	for ev := range out {
		switch ev.Kind {
		case "model_start":
			s.writeSSE(w, "model_start", map[string]string{
				"model_id":     ev.ModelID,
				"display_name": ev.DisplayName,
			})
		case "delta":
			s.writeSSE(w, "delta", map[string]string{
				"model_id": ev.ModelID,
				"delta":    ev.Delta,
			})
		case "model_done":
			if ev.Result != nil {
				results = append(results, *ev.Result)
				s.writeSSE(w, "model_done", jsonResult{
					ModelID:          ev.Result.ModelID,
					DisplayName:      ev.Result.DisplayName,
					OK:               ev.Result.OK,
					Content:          ev.Result.Content,
					Error:            ev.Result.Error,
					LatencyMs:        ev.Result.LatencyMs,
					PromptTokens:     ev.Result.PromptTokens,
					CompletionTokens: ev.Result.CompletionTokens,
					TotalTokens:      ev.Result.TotalTokens,
				})
			}
		case "all_models_done":
			diffs := compare.BuildDiffs(s.cfg.Compare, results)
			for _, d := range diffs {
				s.writeSSE(w, "diff", d)
			}
			s.writeSSE(w, "done", map[string]bool{"ok": true})
		case "error":
			s.writeSSE(w, "error", map[string]string{"message": ev.Error})
		}
		flush(w)
	}
}

func (s *Server) writeSSE(w http.ResponseWriter, event string, data any) {
	var buf bytes.Buffer
	buf.WriteString("event: ")
	buf.WriteString(event)
	buf.WriteString("\n")
	buf.WriteString("data: ")
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(data); err != nil {
		log.Printf("encode sse data: %v", err)
		return
	}
	buf.WriteString("\n")
	if _, err := w.Write(buf.Bytes()); err != nil {
		log.Printf("write sse: %v", err)
	}
}

func flush(w http.ResponseWriter) {
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *Server) resolveProviders(modelIDs []string) ([]provider.Provider, error) {
	if len(modelIDs) == 0 {
		var all []provider.Provider
		for _, m := range s.cfg.EnabledModels() {
			if p, ok := s.providers[m.ID]; ok {
				all = append(all, p)
			}
		}
		if len(all) == 0 {
			return nil, errors.New("no enabled models in configuration")
		}
		return all, nil
	}
	var selected []provider.Provider
	for _, id := range modelIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		m, ok := s.cfg.ModelByID(id)
		if !ok {
			return nil, fmt.Errorf("unknown model_id: %s", id)
		}
		if m.Enabled != nil && !*m.Enabled {
			return nil, fmt.Errorf("model %s is disabled", id)
		}
		p, ok := s.providers[id]
		if !ok {
			return nil, fmt.Errorf("model %s is not available", id)
		}
		selected = append(selected, p)
	}
	if len(selected) == 0 {
		return nil, errors.New("no models selected")
	}
	return selected, nil
}

type modelOption struct {
	ID          string
	DisplayName string
}

type pageData struct {
	Prompt   string
	Models   []modelOption
	Selected []string
	Results  []fanout.ModelResult
	Diffs    []compare.PairDiff
}

func (s *Server) listModelOptions() []modelOption {
	var opts []modelOption
	for _, m := range s.cfg.EnabledModels() {
		opts = append(opts, modelOption{ID: m.ID, DisplayName: m.DisplayName})
	}
	return opts
}

func (s *Server) renderPage(w http.ResponseWriter, data pageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.Execute(w, data); err != nil {
		log.Printf("render template: %v", err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}
