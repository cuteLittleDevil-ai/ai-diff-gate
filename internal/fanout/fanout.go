package fanout

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cuteLittleDevil-ai/ai-diff-gate/internal/provider"
)

// ModelResult is the outcome of a single model invocation.
type ModelResult struct {
	ModelID     string
	DisplayName string
	OK          bool
	Content     string
	Error       string
	LatencyMs   int64
	PromptTokens     *int
	CompletionTokens *int
	TotalTokens      *int
}

// Runner executes concurrent provider calls.
type Runner struct {
	Timeout time.Duration
}

// Run calls each provider with the same user prompt concurrently.
func (r *Runner) Run(ctx context.Context, prompt string, providers []provider.Provider) []ModelResult {
	results := make([]ModelResult, len(providers))
	var wg sync.WaitGroup
	for i, p := range providers {
		wg.Add(1)
		go func(idx int, prov provider.Provider) {
			defer wg.Done()
			results[idx] = r.callOne(ctx, prompt, prov)
		}(i, p)
	}
	wg.Wait()
	return results
}

func (r *Runner) callOne(parent context.Context, prompt string, prov provider.Provider) ModelResult {
	res := ModelResult{
		ModelID:     prov.ID(),
		DisplayName: prov.DisplayName(),
	}

	ctx := parent
	if r.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(parent, r.Timeout)
		defer cancel()
	}

	start := time.Now()
	resp, err := prov.Chat(ctx, &provider.ChatRequest{
		Model: prov.ID(),
		Messages: []provider.Message{
			{Role: "user", Content: prompt},
		},
	})
	latency := time.Since(start).Milliseconds()
	res.LatencyMs = latency

	if err != nil {
		res.OK = false
		res.Error = err.Error()
		if ctx.Err() == context.DeadlineExceeded {
			res.Error = fmt.Sprintf("timeout after %s", r.Timeout)
			res.LatencyMs = r.Timeout.Milliseconds()
		}
		return res
	}

	res.OK = true
	res.Content = resp.Content
	if resp.LatencyMs > 0 {
		res.LatencyMs = resp.LatencyMs
	}
	res.PromptTokens = resp.PromptTokens
	res.CompletionTokens = resp.CompletionTokens
	res.TotalTokens = resp.TotalTokens
	return res
}
