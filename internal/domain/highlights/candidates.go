package highlights

import (
	"strings"
	"time"

	"github.com/forPelevin/hlcut/internal/types"
)

// BuildCandidates creates many candidate windows from the transcript.
// For now: simple segment windows capped by maxClip.
func BuildCandidates(tr types.Transcript, maxClip time.Duration) []types.Candidate {
	segs := tr.Segments
	if len(segs) == 0 {
		return nil
	}

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

func dur(sec float64) time.Duration { return time.Duration(sec * float64(time.Second)) }
