package subtitles

import (
	"fmt"
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

func TestRenderTikTokASS_DoesNotDropTailWords(t *testing.T) {
	words := make([]types.Word, 0, 30)
	for i := 0; i < 30; i++ {
		st := float64(i) * 0.4
		words = append(words, types.Word{
			Start: st,
			End:   st + 0.35,
			Word:  fmt.Sprintf("w%d", i),
		})
	}

	tr := types.Transcript{Segments: []types.Segment{
		{Start: 0, End: 12, Words: words},
	}}
	ass, err := RenderTikTokASS(tr, 0, 12*time.Second)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(ass, "w29") {
		t.Fatalf("expected last word to be present in subtitles, got:\n%s", ass)
	}
	if strings.Count(ass, "Dialogue:") < 3 {
		t.Fatalf("expected multiple dialogue lines instead of truncation, got:\n%s", ass)
	}
}
