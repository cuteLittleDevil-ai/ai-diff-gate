package provider

import (
	"context"
	"strings"
)

// MockProvider is a test double that can simulate both chat and streaming responses.
type MockProvider struct {
	IDVal        string
	DisplayName_ string
	Resp         *ChatResponse
	Err          error
	Deltas       []string
}

func (m *MockProvider) ID() string             { return m.IDVal }
func (m *MockProvider) DisplayName() string      { return m.DisplayName_ }

func (m *MockProvider) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	if m.Resp != nil {
		return m.Resp, nil
	}
	return &ChatResponse{Content: "", Model: m.IDVal}, nil
}

func (m *MockProvider) ChatStream(ctx context.Context, req *ChatRequest, onDelta DeltaFunc) (*ChatResponse, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	var sb strings.Builder
	for _, d := range m.Deltas {
		onDelta(StreamDelta{Content: d})
		sb.WriteString(d)
	}
	if m.Resp != nil {
		resp := *m.Resp
		resp.Content = sb.String()
		return &resp, nil
	}
	return &ChatResponse{Content: sb.String(), Model: m.IDVal}, nil
}
