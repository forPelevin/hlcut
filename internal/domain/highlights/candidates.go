package highlights

import (
	"strings"
	"time"

	"github.com/forPelevin/hlcut/internal/types"
)

// BuildCandidates creates many candidate windows from the transcript.
// MVP strategy:
//   - Prefer word-timestamp-driven windows when available (more granular than segments).
//   - Fall back to segment windows.
func BuildCandidates(tr types.Transcript, maxClip time.Duration) []types.Candidate {
	segs := tr.Segments
	if len(segs) == 0 {
		return nil
	}

	// 1) Word-driven windows (preferred)
	words := collectAllWords(tr)
	if len(words) >= 2 {
		cands := buildFromWords(words, maxClip)
		if len(cands) > 0 {
			return cands
		}
	}

	// 2) Segment-driven fallback
	var out []types.Candidate
	for i := 0; i < len(segs); i++ {
		start := dur(segs[i].Start)
		var parts []string
		end := start
		for j := i; j < len(segs); j++ {
			end = dur(segs[j].End)
			if end-start > maxClip {
				break
			}
			if strings.TrimSpace(segs[j].Text) != "" {
				parts = append(parts, strings.TrimSpace(segs[j].Text))
			}
			text := strings.TrimSpace(strings.Join(parts, " "))
			if text == "" {
				continue
			}
			info, hook := Score(text)
			out = append(out, types.Candidate{Start: start, End: end, Text: text, InfoScore: info, HookScore: hook})
		}
	}
	return out
}

type timedWord struct {
	Start time.Duration
	End   time.Duration
	Text  string
}

func collectAllWords(tr types.Transcript) []timedWord {
	var out []timedWord
	for _, s := range tr.Segments {
		for _, w := range s.Words {
			ws := dur(w.Start)
			we := dur(w.End)
			if we <= ws {
				continue
			}
			text := strings.TrimSpace(w.Word)
			if text == "" {
				continue
			}
			out = append(out, timedWord{Start: ws, End: we, Text: text})
		}
	}
	return out
}

func buildFromWords(words []timedWord, maxClip time.Duration) []types.Candidate {
	// Heuristic caps to avoid O(n^2) blow-ups on long transcripts.
	const (
		maxCandidates = 500
		maxWordsInWin = 40
		startStride   = 3
		endStride     = 6
	)

	var out []types.Candidate
	for i := 0; i < len(words); i += startStride {
		start := words[i].Start
		// Build progressively longer windows.
		for j := i + 1; j < len(words) && j-i <= maxWordsInWin; j += endStride {
			end := words[j].End
			if end-start > maxClip {
				break
			}

			var parts []string
			for k := i; k <= j; k++ {
				parts = append(parts, words[k].Text)
			}
			text := strings.TrimSpace(strings.Join(parts, " "))
			if text == "" {
				continue
			}
			info, hook := Score(text)
			out = append(out, types.Candidate{Start: start, End: end, Text: text, InfoScore: info, HookScore: hook})
			if len(out) >= maxCandidates {
				return out
			}
		}
	}
	return out
}

func dur(sec float64) time.Duration { return time.Duration(sec * float64(time.Second)) }
