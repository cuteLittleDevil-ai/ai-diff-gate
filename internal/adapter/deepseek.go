package adapter

import (
	"net/http"

	"github.com/cuteLittleDevil-ai/ai-diff-gate/internal/modelspec"
	"github.com/cuteLittleDevil-ai/ai-diff-gate/internal/provider"
)

// NewDeepSeek creates a DeepSeek chat provider.
// DeepSeek Chat is compatible with the OpenAI Chat Completions format.
// DeepSeek spec: https://api-docs.deepseek.com/api/create-chat-completion
// OpenAI compatibility reference: https://platform.openai.com/docs/api-reference/chat/create
func NewDeepSeek(cfg modelspec.Model, client *http.Client) provider.Provider {
	return &openAIProvider{cfg: cfg, client: client, name: "deepseek"}
}
