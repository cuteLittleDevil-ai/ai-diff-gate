package fanout

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/cuteLittleDevil-ai/ai-diff-gate/internal/provider"
)

func TestRunStream(t *testing.T) {
	p1 := &provider.MockProvider{
		IDVal:        "model-a",
		DisplayName_: "Model A",
		Deltas:       []string{"Hello", " world"},
	}
	p2 := &provider.MockProvider{
		IDVal:        "model-b",
		DisplayName_: "Model B",
		Deltas:       []string{"Hi", " there"},
	}

	r := &Runner{Timeout: 5 * time.Second}
	out := make(chan StreamEvent, 32)
	go r.RunStream(context.Background(), "hi", []provider.Provider{p1, p2}, out)

	var modelStarts int
	var deltas int
	var modelDone int
	for ev := range out {
		switch ev.Kind {
		case "model_start":
			modelStarts++
		case "delta":
			deltas++
		case "model_done":
			modelDone++
			if ev.Result == nil {
				t.Fatal("model_done missing result")
			}
		case "all_models_done":
			// expected final event
		default:
			t.Fatalf("unexpected event kind: %s", ev.Kind)
		}
	}

	if modelStarts != 2 {
		t.Errorf("model_start events = %d, want 2", modelStarts)
	}
	if deltas != 4 {
		t.Errorf("delta events = %d, want 4", deltas)
	}
	if modelDone != 2 {
		t.Errorf("model_done events = %d, want 2", modelDone)
	}
}

func TestRunStreamError(t *testing.T) {
	p1 := &provider.MockProvider{
		IDVal:        "model-a",
		DisplayName_: "Model A",
		Deltas:       []string{"ok"},
	}
	p2 := &provider.MockProvider{
		IDVal:        "model-b",
		DisplayName_: "Model B",
		Err:          errors.New("boom"),
	}

	r := &Runner{Timeout: 5 * time.Second}
	out := make(chan StreamEvent, 32)
	go r.RunStream(context.Background(), "hi", []provider.Provider{p1, p2}, out)

	var ok, failed bool
	for ev := range out {
		if ev.Kind == "model_done" && ev.Result != nil {
			if ev.Result.OK {
				ok = true
				if !strings.Contains(ev.Result.Content, "ok") {
					t.Errorf("unexpected success content: %q", ev.Result.Content)
				}
			} else {
				failed = true
				if !strings.Contains(ev.Result.Error, "boom") {
					t.Errorf("unexpected error: %q", ev.Result.Error)
				}
			}
		}
	}
	if !ok {
		t.Error("expected one successful model")
	}
	if !failed {
		t.Error("expected one failed model")
	}
}
