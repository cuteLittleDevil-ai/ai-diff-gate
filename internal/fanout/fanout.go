package fanout

import (
	"context"
	"fmt"
	"strings"
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

// StreamEvent is a single event emitted during a streaming comparison.
type StreamEvent struct {
	Kind        string `json:"kind"`
	ModelID     string `json:"model_id,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	Delta       string `json:"delta,omitempty"`
	Result      *ModelResult `json:"result,omitempty"`
	Error       string       `json:"error,omitempty"`
}

// Runner executes concurrent provider calls.
type Runner struct {
	Timeout time.Duration
}

type modelState struct {
	result  ModelResult
	content strings.Builder
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

// RunStream calls each provider concurrently and pushes streaming events to out.
// The caller must consume out until it is closed. After all providers finish,
// an "all_models_done" event is sent and the channel is closed.
func (r *Runner) RunStream(ctx context.Context, prompt string, providers []provider.Provider, out chan<- StreamEvent) {
	defer close(out)
	if len(providers) == 0 {
		sendEvent(out, StreamEvent{Kind: "error", Error: "no models selected"})
		return
	}

	states := make([]*modelState, len(providers))
	for i, p := range providers {
		states[i] = &modelState{result: ModelResult{ModelID: p.ID(), DisplayName: p.DisplayName()}}
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	for i, p := range providers {
		idx := i
		prov := p
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.callOneStream(ctx, prompt, prov, states[idx], out, &mu)
		}()
	}

	wg.Wait()
	for _, s := range states {
		s.result.Content = s.content.String()
	}
	sendEvent(out, StreamEvent{Kind: "all_models_done"})
}

func (r *Runner) callOneStream(parent context.Context, prompt string, prov provider.Provider, state *modelState, out chan<- StreamEvent, mu *sync.Mutex) {
	sendEvent(out, StreamEvent{Kind: "model_start", ModelID: prov.ID(), DisplayName: prov.DisplayName()})

	ctx := parent
	if r.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(parent, r.Timeout)
		defer cancel()
	}

	start := time.Now()
	onDelta := func(d provider.StreamDelta) {
		if d.Content == "" {
			return
		}
		mu.Lock()
		state.content.WriteString(d.Content)
		mu.Unlock()
		sendEvent(out, StreamEvent{Kind: "delta", ModelID: prov.ID(), Delta: d.Content})
	}

	resp, err := prov.ChatStream(ctx, &provider.ChatRequest{
		Model: prov.ID(),
		Messages: []provider.Message{
			{Role: "user", Content: prompt},
		},
	}, onDelta)

	latency := time.Since(start).Milliseconds()

	mu.Lock()
	state.result.LatencyMs = latency
	if err != nil {
		state.result.OK = false
		state.result.Error = err.Error()
		if ctx.Err() == context.DeadlineExceeded {
			state.result.Error = fmt.Sprintf("timeout after %s", r.Timeout)
			state.result.LatencyMs = r.Timeout.Milliseconds()
		}
	} else {
		state.result.OK = true
		state.result.Content = state.content.String()
		if resp != nil {
			if resp.LatencyMs > 0 {
				state.result.LatencyMs = resp.LatencyMs
			}
			state.result.PromptTokens = resp.PromptTokens
			state.result.CompletionTokens = resp.CompletionTokens
			state.result.TotalTokens = resp.TotalTokens
		}
	}
	res := state.result
	mu.Unlock()

	sendEvent(out, StreamEvent{Kind: "model_done", ModelID: prov.ID(), DisplayName: prov.DisplayName(), Result: &res})
}

func sendEvent(out chan<- StreamEvent, ev StreamEvent) {
	select {
	case out <- ev:
	case <-time.After(5 * time.Second):
	}
}
