package adapter

import (
	"bufio"
	"io"
	"strings"
)

// sseEvent is a parsed Server-Sent Event.
type sseEvent struct {
	Event string
	Data  string
}

// forEachSSE reads a text/event-stream and invokes cb for each complete event.
func forEachSSE(r io.Reader, cb func(ev sseEvent) error) error {
	scanner := bufio.NewScanner(r)
	var event, data strings.Builder
	emit := func() error {
		if data.Len() == 0 && event.Len() == 0 {
			return nil
		}
		ev := sseEvent{Event: event.String(), Data: data.String()}
		event.Reset()
		data.Reset()
		return cb(ev)
	}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if err := emit(); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, "event:") {
			event.Reset()
			event.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "event:")))
		} else if strings.HasPrefix(line, "data:") {
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return emit()
}
