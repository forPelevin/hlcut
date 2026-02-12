package highlights

import (
	"regexp"
	"strings"
)

var (
	reNum     = regexp.MustCompile(`\b\d+(?:[\.,]\d+)?\b`)
	reHook    = regexp.MustCompile(`(?i)\b(important|key|secret|mistake|never|always|here\s+is\s+why|remember)\b`)
	reHow     = regexp.MustCompile(`(?i)\b(how\s+to|step\s+\d+|first|second|third|do\s+this)\b`)
	reStepNum = regexp.MustCompile(`(?i)\bstep\s+\d+\b`)
)

// Score returns (info, hook) in range [0..10].
func Score(text string) (float64, float64) {
	t := strings.TrimSpace(text)
	if t == "" {
		return 0, 0
	}
	lower := strings.ToLower(t)

	// Lightweight heuristic on purpose: deterministic, cheap, and "good enough"
	// for candidate pre-ranking before LLM refinement makes final selections.
	info := float64(len(reNum.FindAllStringIndex(t, -1))) * 0.4
	if reHow.MatchString(lower) {
		info += 1.2
	}
	// small length penalty
	info -= 0.0006 * float64(len([]rune(t)))

	hook := float64(len(reHook.FindAllStringIndex(lower, -1))) * 0.9
	// Procedural step numbers tend to retain attention ("Step 1", "Step 2", ...).
	hook += float64(len(reStepNum.FindAllStringIndex(lower, -1))) * 0.4
	hook += float64(strings.Count(t, "?")) * 0.7
	hook += float64(strings.Count(t, "!")) * 0.3

	return clamp(info, 0, 10), clamp(hook, 0, 10)
}

func clamp(x, a, b float64) float64 {
	if x < a {
		return a
	}
	if x > b {
		return b
	}
	return x
}
