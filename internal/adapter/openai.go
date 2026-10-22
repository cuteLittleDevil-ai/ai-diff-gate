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
	name   string
}

// NewOpenAI creates an OpenAI-compatible chat provider.
// Spec: https://platform.openai.com/docs/api-reference/chat/create
func NewOpenAI(cfg modelspec.Model, client *http.Client) provider.Provider {
	return &openAIProvider{cfg: cfg, client: client, name: "openai"}
}

func (p *openAIProvider) ID() string          { return p.cfg.ID }
func (p *openAIProvider) DisplayName() string { return p.cfg.DisplayName }

type openAIRequest struct {
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"`
	Stream   bool            `json:"stream,omitempty"`
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
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
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
	return p.chat(ctx, req, false, nil)
}

func (p *openAIProvider) ChatStream(ctx context.Context, req *provider.ChatRequest, onDelta provider.DeltaFunc) (*provider.ChatResponse, error) {
	return p.chat(ctx, req, true, onDelta)
}

func (p *openAIProvider) chat(ctx context.Context, req *provider.ChatRequest, stream bool, onDelta provider.DeltaFunc) (*provider.ChatResponse, error) {
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
		Stream:   stream,
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
		return nil, fmt.Errorf("%s API %s: %s", p.name, resp.Status, truncate(string(raw), 512))
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

func (p *openAIProvider) readStream(r io.Reader, latency int64, model string, onDelta provider.DeltaFunc) (*provider.ChatResponse, error) {
	var content strings.Builder
	err := forEachSSE(r, func(ev sseEvent) error {
		if ev.Data == "[DONE]" {
			return nil
		}
		if ev.Data == "" {
			return nil
		}
		var parsed openAIResponse
		if err := json.Unmarshal([]byte(ev.Data), &parsed); err != nil {
			return fmt.Errorf("decode %s stream chunk: %w", p.name, err)
		}
		if parsed.Error != nil && parsed.Error.Message != "" {
			return fmt.Errorf("%s stream error: %s", p.name, parsed.Error.Message)
		}
		if len(parsed.Choices) > 0 {
			delta := parsed.Choices[0].Delta.Content
			if delta != "" {
				content.WriteString(delta)
				if onDelta != nil {
					onDelta(provider.StreamDelta{Content: delta})
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &provider.ChatResponse{
		Content:   content.String(),
		Model:     model,
		LatencyMs: latency,
	}, nil
}

func (p *openAIProvider) parseResponse(raw []byte, latency int64, model string) (*provider.ChatResponse, error) {
	var parsed openAIResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("decode %s response: %w", p.name, err)
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return nil, fmt.Errorf("%s error: %s", p.name, parsed.Error.Message)
	}
	content := ""
	if len(parsed.Choices) > 0 {
		content = parsed.Choices[0].Message.Content
	}

	out := &provider.ChatResponse{
		Content:   content,
		Model:     firstNonEmpty(parsed.Model, model),
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
