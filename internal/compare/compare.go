package compare

import (
	"strings"

	"github.com/cuteLittleDevil-ai/ai-diff-gate/internal/config"
	"github.com/cuteLittleDevil-ai/ai-diff-gate/internal/fanout"
	"github.com/sergi/go-diff/diffmatchpatch"
)

// PairDiff is a unified diff between two successful model outputs.
type PairDiff struct {
	LeftModelID  string `json:"left_model_id"`
	RightModelID string `json:"right_model_id"`
	Granularity  string `json:"granularity"`
	UnifiedDiff  string `json:"unified_diff"`
}

// BuildDiffs generates pairwise unified diffs for all successful results.
func BuildDiffs(cfg config.CompareConfig, results []fanout.ModelResult) []PairDiff {
	var ok []fanout.ModelResult
	for _, r := range results {
		if r.OK {
			ok = append(ok, r)
		}
	}
	if len(ok) < 2 {
		return nil
	}

	granularity := cfg.Granularity
	if granularity == "" {
		granularity = "line"
	}

	var diffs []PairDiff
	for i := 0; i < len(ok); i++ {
		for j := i + 1; j < len(ok); j++ {
			left, right := ok[i], ok[j]
			ud := unifiedDiff(left.Content, right.Content, left.ModelID, right.ModelID, granularity, cfg.IgnoreWhitespace)
			diffs = append(diffs, PairDiff{
				LeftModelID:  left.ModelID,
				RightModelID: right.ModelID,
				Granularity:  granularity,
				UnifiedDiff:  ud,
			})
		}
	}
	return diffs
}

func unifiedDiff(a, b, fromLabel, toLabel, granularity string, ignoreWS bool) string {
	if ignoreWS {
		a = collapseWhitespace(a)
		b = collapseWhitespace(b)
	}

	switch granularity {
	case "word":
		return wordUnifiedDiff(a, b, fromLabel, toLabel)
	default:
		return lineUnifiedDiff(a, b, fromLabel, toLabel)
	}
}

func lineUnifiedDiff(a, b, fromLabel, toLabel string) string {
	linesA := splitLines(a)
	linesB := splitLines(b)
	var out strings.Builder
	out.WriteString("--- " + fromLabel + "\n")
	out.WriteString("+++ " + toLabel + "\n")
	max := len(linesA)
	if len(linesB) > max {
		max = len(linesB)
	}
	for i := 0; i < max; i++ {
		la, lb := "", ""
		if i < len(linesA) {
			la = linesA[i]
		}
		if i < len(linesB) {
			lb = linesB[i]
		}
		if la == lb {
			if la != "" || i < max-1 {
				out.WriteString(" " + la + "\n")
			}
		} else {
			if la != "" || i < len(linesA) {
				out.WriteString("-" + la + "\n")
			}
			if lb != "" || i < len(linesB) {
				out.WriteString("+" + lb + "\n")
			}
		}
	}
	return out.String()
}

func wordUnifiedDiff(a, b, fromLabel, toLabel string) string {
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(a, b, false)
	var out strings.Builder
	out.WriteString("--- " + fromLabel + "\n")
	out.WriteString("+++ " + toLabel + "\n")
	for _, d := range diffs {
		switch d.Type {
		case diffmatchpatch.DiffDelete:
			out.WriteString("-" + d.Text)
		case diffmatchpatch.DiffInsert:
			out.WriteString("+" + d.Text)
		case diffmatchpatch.DiffEqual:
			out.WriteString(" " + d.Text)
		}
	}
	return out.String()
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.Split(s, "\n")
}

func collapseWhitespace(s string) string {
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}
