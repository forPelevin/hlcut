package subtitles

import (
	"strings"
	"testing"
	"time"

	"github.com/forPelevin/hlcut/internal/types"
)

func TestRenderTikTokASS_KaraokeHasKTags(t *testing.T) {
	tr := types.Transcript{Segments: []types.Segment{
		{Start: 0, End: 2, Words: []types.Word{{Start: 0.0, End: 0.3, Word: "Hello"}, {Start: 0.3, End: 0.8, Word: "world"}}},
	}}
	ass, err := RenderTikTokASS(tr, 0, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ass, "{\\k") {
		t.Fatalf("expected karaoke tags in ASS, got:\n%s", ass)
	}
}

func TestAssTime_Format(t *testing.T) {
	got := assTime(61*time.Second + 234*time.Millisecond)
	if got != "0:01:01.23" {
		t.Fatalf("unexpected assTime: %s", got)
	}
}
