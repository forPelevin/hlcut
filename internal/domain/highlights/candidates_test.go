package highlights

import (
	"fmt"
	"testing"
	"time"

	"github.com/forPelevin/hlcut/internal/types"
)

func TestBuildCandidates_RespectsMaxClip(t *testing.T) {
	tr := types.Transcript{Segments: []types.Segment{
		{Start: 0, End: 40, Text: "A"},
		{Start: 40, End: 90, Text: "B"},
	}}
	min, max := DurationBounds()
	cands := BuildCandidates(tr)
	if len(cands) == 0 {
		t.Fatalf("expected candidates")
	}
	for _, c := range cands {
		if c.End-c.Start < min {
			t.Fatalf("candidate under min: %v", c.End-c.Start)
		}
		if c.End-c.Start > max {
			t.Fatalf("candidate exceeds max: %v", c.End-c.Start)
		}
	}
}

func TestBuildCandidates_CoversLateTranscriptParts(t *testing.T) {
	words := make([]types.Word, 0, 300)
	for i := 0; i < 300; i++ {
		// Keep words sparse so each start index contributes a small number of
		// candidates and the global candidate cap does not starve late starts.
		st := float64(i) * 10
		words = append(words, types.Word{
			Start: st,
			End:   st + 0.5,
			Word:  fmt.Sprintf("w%d", i),
		})
	}

	tr := types.Transcript{
		Segments: []types.Segment{
			{Start: 0, End: 3000, Words: words},
		},
	}

	cands := BuildCandidates(tr)
	if len(cands) == 0 {
		t.Fatalf("expected candidates")
	}

	var hasLate bool
	for _, c := range cands {
		if c.Start >= 20*time.Minute {
			hasLate = true
			break
		}
	}
	if !hasLate {
		t.Fatalf("expected candidates from later timeline, got %d candidates", len(cands))
	}
}
