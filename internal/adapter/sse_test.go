package adapter

import (
	"strings"
	"testing"
)

func TestForEachSSE(t *testing.T) {
	input := `data: {"x":1}

data: {"x":2}

event: done
data: [DONE]

`
	var events []sseEvent
	err := forEachSSE(strings.NewReader(input), func(ev sseEvent) error {
		events = append(events, ev)
		return nil
	})
	if err != nil {
		t.Fatalf("forEachSSE: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("events = %d, want 3: %+v", len(events), events)
	}
	if events[0].Data != `{"x":1}` {
		t.Errorf("event[0].data = %q", events[0].Data)
	}
	if events[1].Data != `{"x":2}` {
		t.Errorf("event[1].data = %q", events[1].Data)
	}
	if events[2].Event != "done" || events[2].Data != "[DONE]" {
		t.Errorf("event[2] = %+v", events[2])
	}
}
