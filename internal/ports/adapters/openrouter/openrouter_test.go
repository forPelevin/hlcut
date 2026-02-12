package openrouter

import (
	"testing"
	"time"

	"github.com/forPelevin/hlcut/internal/types"
)

func TestExtractJSONObject(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantSub string
		wantErr bool
	}{
		{"raw", `{"clips":[{"idx":0,"start_sec":0,"end_sec":1,"title":"t","caption":"c","tags":[],"reason":"r"}]}`, `"clips"`, false},
		{"fenced", "```json\n{\"clips\":[]}\n```", `"clips"`, false},
		{"preface", "sure! {\"clips\":[]} thanks", `"clips"`, false},
		{"empty", "   ", "", true},
		{"nojson", "hello", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractJSONObject(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantSub != "" && !contains(got, tt.wantSub) {
				t.Fatalf("expected %q to contain %q", got, tt.wantSub)
			}
		})
	}
}

func TestRedactSecrets(t *testing.T) {
	apiKey := "sk-or-v1-super-secret"
	in := `status 401; Authorization: Bearer sk-or-v1-super-secret; api_key=sk-or-v1-super-secret`
	got := redactSecrets(in, apiKey)

	if contains(got, apiKey) {
		t.Fatalf("expected API key to be redacted, got: %q", got)
	}
	if !contains(got, "Authorization: [REDACTED]") {
		t.Fatalf("expected authorization header to be redacted, got: %q", got)
	}
	if !contains(got, "api_key=[REDACTED]") {
		t.Fatalf("expected api_key field to be redacted, got: %q", got)
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && (index(s, sub) >= 0))
}

func index(s, sub string) int {
	// tiny helper to avoid importing strings just for tests
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestNormalizeClipRange_FallsBackToCandidateIdx(t *testing.T) {
	cands := []types.Candidate{
		{
			Start: 40 * time.Second,
			End:   70 * time.Second,
		},
	}

	st, en, ok := normalizeClipRange(
		0,
		2,
		4,
		cands,
		20*time.Second,
		60*time.Second,
		transcriptTiming{},
	)
	if !ok {
		t.Fatalf("expected clip to normalize")
	}
	if st != 40*time.Second || en != 70*time.Second {
		t.Fatalf("unexpected normalized range: %v -> %v", st, en)
	}
}

func TestFallbackHighlights_DoesNotReturnOverlappingClips(t *testing.T) {
	cands := []types.Candidate{
		{Start: 0, End: 25 * time.Second, Text: "A", InfoScore: 9},
		{Start: 10 * time.Second, End: 35 * time.Second, Text: "B", InfoScore: 8},
		{Start: 36 * time.Second, End: 62 * time.Second, Text: "C", InfoScore: 7},
	}

	out := fallbackHighlights(cands, 3, 20*time.Second, 60*time.Second, transcriptTiming{})
	if len(out) != 2 {
		t.Fatalf("expected 2 non-overlapping clips, got %d", len(out))
	}
	if out[0].End > out[1].Start {
		t.Fatalf("expected non-overlap, got %v and %v", out[0], out[1])
	}
}

func TestNormalizeClipDur_SnapsToPunctuationNearTail(t *testing.T) {
	timing := transcriptTiming{
		words: []timedWord{
			{Start: 53 * time.Second, End: 54 * time.Second, Text: "almost"},
			{Start: 54 * time.Second, End: 55 * time.Second, Text: "there"},
			{Start: 55 * time.Second, End: 56 * time.Second, Text: "finished."},
			{Start: 56 * time.Second, End: 57 * time.Second, Text: "next"},
		},
	}

	_, en, ok := normalizeClipDur(0, 60*time.Second, 20*time.Second, 60*time.Second, timing)
	if !ok {
		t.Fatalf("expected normalized clip")
	}
	if en != 56*time.Second {
		t.Fatalf("expected clip end to snap to punctuation at 56s, got %v", en)
	}
}

func TestNormalizeClipDur_PrefersComprehensiveSentenceEnd(t *testing.T) {
	timing := transcriptTiming{
		words: []timedWord{
			{Start: 53 * time.Second, End: 54 * time.Second, Text: "What"},
			{Start: 54 * time.Second, End: 55 * time.Second, Text: "is"},
			{Start: 55 * time.Second, End: 56 * time.Second, Text: "going"},
			{Start: 56 * time.Second, End: 57 * time.Second, Text: "on?"},
			{Start: 57 * time.Second, End: 57*time.Second + 200*time.Millisecond, Text: "I"},
			{Start: 57*time.Second + 200*time.Millisecond, End: 58 * time.Second, Text: "am"},
			{Start: 58 * time.Second, End: 59 * time.Second, Text: "out."},
		},
	}

	_, en, ok := normalizeClipDur(0, 58*time.Second, 20*time.Second, 60*time.Second, timing)
	if !ok {
		t.Fatalf("expected normalized clip")
	}
	if en != 59*time.Second {
		t.Fatalf("expected clip end to prefer full-resolution sentence at 59s, got %v", en)
	}
}

func TestNormalizeClipDur_AvoidsQuestionTailWhenContinuationStartsImmediately(t *testing.T) {
	timing := transcriptTiming{
		words: []timedWord{
			{Start: 73 * time.Second, End: 74 * time.Second, Text: "there's"},
			{Start: 74 * time.Second, End: 75 * time.Second, Text: "more!"},
			{Start: 75 * time.Second, End: 76 * time.Second, Text: "what"},
			{Start: 76 * time.Second, End: 77 * time.Second, Text: "is"},
			{Start: 77 * time.Second, End: 78 * time.Second, Text: "going"},
			{Start: 78 * time.Second, End: 79 * time.Second, Text: "on?"},
			{Start: 79 * time.Second, End: 79*time.Second + 200*time.Millisecond, Text: "i"},
			{Start: 79*time.Second + 200*time.Millisecond, End: 80 * time.Second, Text: "was"},
		},
	}

	// start=20s, max=60s => hard upper bound is 80s, so "on?" is the tail candidate;
	// the logic should back off to "more!" to avoid abrupt unresolved question ending.
	_, en, ok := normalizeClipDur(20*time.Second, 80*time.Second, 20*time.Second, 60*time.Second, timing)
	if !ok {
		t.Fatalf("expected normalized clip")
	}
	if en != 75*time.Second {
		t.Fatalf("expected clip end to back off to 75s for smoother closure, got %v", en)
	}
}
