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
func BuildCandidates(tr types.Transcript, minClip, maxClip time.Duration) []types.Candidate {
	// Guardrails keep callers safe from bad config while preserving a useful
	// lower bound for clip duration.
	if minClip <= 0 {
		minClip = time.Second
	}
	if maxClip <= 0 || maxClip < minClip {
		return nil
	}

	segs := tr.Segments
	if len(segs) == 0 {
		return nil
	}

	// 1) Word-driven windows (preferred): gives tighter boundaries and better
	// candidate text slices for downstream ranking/refinement.
	words := collectAllWords(tr)
	if len(words) >= 2 {
		cands := buildFromWords(words, minClip, maxClip)
		if len(cands) > 0 {
			return cands
		}
	}

	// 2) Segment-driven fallback: keeps pipeline functional for ASR outputs that
	// do not include reliable word timestamps.
	var out []types.Candidate
	for i := 0; i < len(segs); i++ {
		start := dur(segs[i].Start)
		var parts []string
		for j := i; j < len(segs); j++ {
			end := dur(segs[j].End)
			win := end - start
			if win > maxClip {
				break
			}
			if win < minClip {
				continue
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

func buildFromWords(words []timedWord, minClip, maxClip time.Duration) []types.Candidate {
	// Heuristic caps keep runtime predictable on long transcripts without fully
	// sacrificing timeline coverage.
	const (
		maxCandidates = 500
		maxWordsInWin = 240
		maxStartCount = 140
		endStride     = 4
	)

	startStride := 1
	if len(words) > maxStartCount {
		startStride = (len(words) + maxStartCount - 1) / maxStartCount
	}
	startIdxs := make([]int, 0, len(words)/startStride+2)
	for i := 0; i < len(words)-1; i += startStride {
		startIdxs = append(startIdxs, i)
	}
	lastStart := len(words) - 2
	// Always include a near-tail start index so late parts of the transcript
	// still contribute candidates even when start indices are downsampled.
	if lastStart >= 0 && (len(startIdxs) == 0 || startIdxs[len(startIdxs)-1] != lastStart) {
		startIdxs = append(startIdxs, lastStart)
	}

	var out []types.Candidate
	for _, i := range startIdxs {
		start := words[i].Start

		parts := make([]string, 0, maxWordsInWin)
		// Build progressively longer windows; stride on end indices reduces work
		// while still exploring short and medium windows from each start point.
		for j := i; j < len(words) && j-i <= maxWordsInWin; j++ {
			parts = append(parts, words[j].Text)
			if j == i {
				continue
			}
			if (j-i)%endStride != 0 && j != i+1 {
				continue
			}

			end := words[j].End
			win := end - start
			if win > maxClip {
				break
			}
			if win < minClip {
				continue
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
