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

type anthropicProvider struct {
	cfg    modelspec.Model
	client *http.Client
}

// NewAnthropic creates an Anthropic Messages API provider.
// Spec: https://docs.anthropic.com/en/api/messages
func NewAnthropic(cfg modelspec.Model, client *http.Client) provider.Provider {
	return &anthropicProvider{cfg: cfg, client: client}
}

func (p *anthropicProvider) ID() string          { return p.cfg.ID }
func (p *anthropicProvider) DisplayName() string { return p.cfg.DisplayName }

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	Messages  []anthropicMessage `json:"messages"`
	Stream    bool               `json:"stream,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Model   string `json:"model"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Usage *struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

type anthropicStreamEvent struct {
	Type      string `json:"type"`
	Index     int    `json:"index"`
	ContentBlock *struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content_block"`
	Delta *struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta"`
	Usage *struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func (p *anthropicProvider) Chat(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	return p.chat(ctx, req, false, nil)
}

func (p *anthropicProvider) ChatStream(ctx context.Context, req *provider.ChatRequest, onDelta provider.DeltaFunc) (*provider.ChatResponse, error) {
	return p.chat(ctx, req, true, onDelta)
}

func (p *anthropicProvider) chat(ctx context.Context, req *provider.ChatRequest, stream bool, onDelta provider.DeltaFunc) (*provider.ChatResponse, error) {
	key, err := p.cfg.ResolveAPIKey()
	if err != nil {
		return nil, err
	}

	msgs := make([]anthropicMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		role := m.Role
		if role == "system" {
			role = "user"
		}
		msgs = append(msgs, anthropicMessage{Role: role, Content: m.Content})
	}

	model := p.cfg.Model
	if req.Model != "" && req.Model != p.cfg.ID {
		model = req.Model
	}
	body, err := json.Marshal(anthropicRequest{
		Model:     model,
		MaxTokens: 1024,
		Messages:  msgs,
		Stream:    stream,
	})
	if err != nil {
		return nil, err
	}

	base := strings.TrimRight(p.cfg.BaseURL, "/")
	url := base + "/v1/messages"

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", key)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	if stream {
		httpReq.Header.Set("Accept", "text/event-stream")
	}

	start := time.Now()
	resp, err := p.client.Do(httpReq)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("anthropic API %s: %s", resp.Status, truncate(string(raw), 512))
	}

	if stream {
		return p.readStream(resp.Body, latency, req.Model, onDelta)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return p.parseResponse(raw, latency, req.Model)
}

func (p *anthropicProvider) readStream(r io.Reader, latency int64, model string, onDelta provider.DeltaFunc) (*provider.ChatResponse, error) {
	var content strings.Builder
	var promptTokens, completionTokens int
	err := forEachSSE(r, func(ev sseEvent) error {
		if ev.Data == "" {
			return nil
		}
		var parsed anthropicStreamEvent
		if err := json.Unmarshal([]byte(ev.Data), &parsed); err != nil {
			return fmt.Errorf("decode anthropic stream chunk: %w", err)
		}
		if parsed.Error != nil && parsed.Error.Message != "" {
			return fmt.Errorf("anthropic stream error: %s", parsed.Error.Message)
		}
		switch parsed.Type {
		case "content_block_delta":
			if parsed.Delta != nil && parsed.Delta.Text != "" {
				content.WriteString(parsed.Delta.Text)
				if onDelta != nil {
					onDelta(provider.StreamDelta{Content: parsed.Delta.Text})
				}
			}
		case "message_delta":
			if parsed.Usage != nil {
				promptTokens = parsed.Usage.InputTokens
				completionTokens = parsed.Usage.OutputTokens
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	out := &provider.ChatResponse{
		Content:   content.String(),
		Model:     model,
		LatencyMs: latency,
	}
	if promptTokens > 0 || completionTokens > 0 {
		pt, ct := promptTokens, completionTokens
		tt := pt + ct
		out.PromptTokens = &pt
		out.CompletionTokens = &ct
		out.TotalTokens = &tt
	}
	return out, nil
}

func (p *anthropicProvider) parseResponse(raw []byte, latency int64, model string) (*provider.ChatResponse, error) {
	var parsed anthropicResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("decode anthropic response: %w", err)
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return nil, fmt.Errorf("anthropic error: %s", parsed.Error.Message)
	}

	var parts []string
	for _, block := range parsed.Content {
		if block.Type == "text" {
			parts = append(parts, block.Text)
		}
	}

	out := &provider.ChatResponse{
		Content:   strings.Join(parts, ""),
		Model:     firstNonEmpty(parsed.Model, model),
		LatencyMs: latency,
	}
	if parsed.Usage != nil {
		pt, ct := parsed.Usage.InputTokens, parsed.Usage.OutputTokens
		tt := pt + ct
		out.PromptTokens = &pt
		out.CompletionTokens = &ct
		out.TotalTokens = &tt
	}
	return out, nil
}
