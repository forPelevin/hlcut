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
	min := 20 * time.Second
	max := 60 * time.Second
	cands := BuildCandidates(tr, min, max)
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
		st := float64(i) * 0.5
		words = append(words, types.Word{
			Start: st,
			End:   st + 0.5,
			Word:  fmt.Sprintf("w%d", i),
		})
	}

	tr := types.Transcript{
		Segments: []types.Segment{
			{Start: 0, End: 150, Words: words},
		},
	}

	cands := BuildCandidates(tr, 20*time.Second, 30*time.Second)
	if len(cands) == 0 {
		t.Fatalf("expected candidates")
	}

	var hasLate bool
	for _, c := range cands {
		if c.Start >= 90*time.Second {
			hasLate = true
			break
		}
	}
	if !hasLate {
		t.Fatalf("expected candidates from later timeline, got %d candidates", len(cands))
	}
}
