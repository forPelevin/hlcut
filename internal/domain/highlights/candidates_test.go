package highlights

import (
	"testing"
	"time"

	"github.com/forPelevin/hlcut/internal/types"
)

func TestBuildCandidates_RespectsMaxClip(t *testing.T) {
	tr := types.Transcript{Segments: []types.Segment{
		{Start: 0, End: 40, Text: "A"},
		{Start: 40, End: 90, Text: "B"},
	}}
	max := 60 * time.Second
	cands := BuildCandidates(tr, max)
	if len(cands) == 0 {
		t.Fatalf("expected candidates")
	}
	for _, c := range cands {
		if c.End-c.Start > max {
			t.Fatalf("candidate exceeds max: %v", c.End-c.Start)
		}
	}
}
