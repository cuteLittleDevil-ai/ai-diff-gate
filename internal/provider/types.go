package provider

import "context"

// Message is a single chat message in the internal normalized form.
type Message struct {
	Role    string
	Content string
}

// ChatRequest is the normalized request passed to all providers.
type ChatRequest struct {
	Model    string
	Messages []Message
}

// ChatResponse is the normalized response from a provider call.
type ChatResponse struct {
	Content          string
	Model            string
	LatencyMs        int64
	PromptTokens     *int
	CompletionTokens *int
	TotalTokens      *int
}

// Provider is a single configured model endpoint.
type Provider interface {
	ID() string
	DisplayName() string
	Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
}
