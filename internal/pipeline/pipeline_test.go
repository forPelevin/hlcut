package pipeline

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestBuildRunOutDir(t *testing.T) {
	now := time.Date(2026, 2, 12, 10, 30, 45, 1234, time.UTC)
	got := buildRunOutDir("out", "/tmp/My Cool.Video.mp4", now)
	base := filepath.Base(got)
	if filepath.Dir(got) != "out" {
		t.Fatalf("unexpected parent dir: %s", got)
	}
	tsPrefix := fmt.Sprintf("%013d", now.UTC().UnixMilli())
	if !strings.HasPrefix(base, tsPrefix+"-my-cool-video-") {
		t.Fatalf("unexpected run dir format: %s", base)
	}
	if len(base) != len(tsPrefix+"-my-cool-video-")+6 {
		t.Fatalf("unexpected run dir suffix length: %s", base)
	}
	parts := strings.SplitN(base, "-", 3)
	if len(parts) != 3 {
		t.Fatalf("unexpected run dir parts: %s", base)
	}
	if len(parts[0]) != 13 {
		t.Fatalf("timestamp must be 13 digits, got %q", parts[0])
	}
	if _, err := strconv.ParseInt(parts[0], 10, 64); err != nil {
		t.Fatalf("timestamp must be numeric, got %q: %v", parts[0], err)
	}
}

func TestBuildRunOutDirLexicographicOrderByTime(t *testing.T) {
	earlier := time.Date(2026, 2, 12, 10, 30, 45, 0, time.UTC)
	later := earlier.Add(time.Millisecond)

	// Names are intentionally reverse-alphabetical to ensure time prefix drives sorting.
	a := filepath.Base(buildRunOutDir("out", "/tmp/zeta.mp4", earlier))
	b := filepath.Base(buildRunOutDir("out", "/tmp/alpha.mp4", later))

	if a >= b {
		t.Fatalf("expected earlier run dir to sort first: %q >= %q", a, b)
	}
}

func TestNormalizePathSegment(t *testing.T) {
	tests := map[string]string{
		"  My Cool.Video  ": "my-cool-video",
		"___":               "",
		"abc123":            "abc123",
		"Name (v2)!":        "name-v2",
	}
	for in, want := range tests {
		t.Run(in, func(t *testing.T) {
			if got := normalizePathSegment(in); got != want {
				t.Fatalf("normalizePathSegment(%q) = %q, want %q", in, got, want)
			}
		})
	}
}

func TestAutoClipCount(t *testing.T) {
	tests := []struct {
		name     string
		base     int
		duration time.Duration
		want     int
	}{
		{
			name:     "keeps base for short videos",
			base:     12,
			duration: 18 * time.Minute,
			want:     12,
		},
		{
			name:     "increases when duration implies higher cap",
			base:     12,
			duration: 61 * time.Minute,
			want:     13,
		},
		{
			name:     "rounds up partial windows",
			base:     12,
			duration: 65 * time.Minute,
			want:     13,
		},
		{
			name:     "respects hard max limit",
			base:     12,
			duration: 12 * time.Hour,
			want:     autoClipMaxLimit,
		},
		{
			name:     "keeps explicit higher base",
			base:     20,
			duration: 70 * time.Minute,
			want:     20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := autoClipCount(tt.base, tt.duration)
			if got != tt.want {
				t.Fatalf("autoClipCount(%d, %s) = %d, want %d", tt.base, tt.duration, got, tt.want)
			}
		})
	}
}
