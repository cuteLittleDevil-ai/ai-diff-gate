package adapter

import (
	"net/http"

	"github.com/cuteLittleDevil-ai/ai-diff-gate/internal/modelspec"
	"github.com/cuteLittleDevil-ai/ai-diff-gate/internal/provider"
)

// NewMiniMax creates a MiniMax chat provider.
// MiniMax supports both OpenAI-compatible and Anthropic-compatible endpoints;
// this adapter uses the OpenAI Chat Completions format.
//
// Spec:
//   - OpenAI compatibility overview: https://platform.minimax.io/docs/api-reference/text-openai-api
//   - Chat Completions: https://platform.minimax.io/docs/api-reference/text-chat-openai
//   - Text generation guide: https://platform.minimax.io/docs/guides/text-generation
//   - Anthropic compatibility (not implemented here): https://platform.minimax.io/docs/api-reference/text-anthropic-api
//
// OpenAI reference: https://platform.openai.com/docs/api-reference/chat/create
func NewMiniMax(cfg modelspec.Model, client *http.Client) provider.Provider {
	return &openAIProvider{cfg: cfg, client: client, name: "minimax"}
}
