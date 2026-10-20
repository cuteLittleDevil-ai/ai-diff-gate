package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cuteLittleDevil-ai/ai-diff-gate/internal/modelspec"
	"github.com/cuteLittleDevil-ai/ai-diff-gate/internal/provider"
)

type openAIProvider struct {
	cfg    modelspec.Model
	client *http.Client
}

// NewOpenAI creates an OpenAI-compatible chat provider.
func NewOpenAI(cfg modelspec.Model, client *http.Client) provider.Provider {
	return &openAIProvider{cfg: cfg, client: client}
}

func (p *openAIProvider) ID() string          { return p.cfg.ID }
func (p *openAIProvider) DisplayName() string { return p.cfg.DisplayName }

type openAIRequest struct {
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (p *openAIProvider) Chat(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	key, err := p.cfg.ResolveAPIKey()
	if err != nil {
		return nil, err
	}

	msgs := make([]openAIMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		msgs = append(msgs, openAIMessage{Role: m.Role, Content: m.Content})
	}
	model := p.cfg.Model
	if req.Model != "" && req.Model != p.cfg.ID {
		model = req.Model
	}
	body, err := json.Marshal(openAIRequest{
		Model:    model,
		Messages: msgs,
	})
	if err != nil {
		return nil, err
	}

	base := strings.TrimRight(p.cfg.BaseURL, "/")
	url := base + "/chat/completions"

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+key)

	start := time.Now()
	resp, err := p.client.Do(httpReq)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("openai API %s: %s", resp.Status, truncate(string(raw), 512))
	}

	var parsed openAIResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("decode openai response: %w", err)
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return nil, fmt.Errorf("openai error: %s", parsed.Error.Message)
	}
	content := ""
	if len(parsed.Choices) > 0 {
		content = parsed.Choices[0].Message.Content
	}

	out := &provider.ChatResponse{
		Content:   content,
		Model:     firstNonEmpty(parsed.Model, req.Model),
		LatencyMs: latency,
	}
	if parsed.Usage != nil {
		pt, ct, tt := parsed.Usage.PromptTokens, parsed.Usage.CompletionTokens, parsed.Usage.TotalTokens
		out.PromptTokens = &pt
		out.CompletionTokens = &ct
		out.TotalTokens = &tt
	}
	return out, nil
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
