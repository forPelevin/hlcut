package pipeline

import (
	"path/filepath"
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
	if !strings.HasPrefix(base, "my-cool-video-20260212-103045Z-") {
		t.Fatalf("unexpected run dir format: %s", base)
	}
	if len(base) != len("my-cool-video-20260212-103045Z-")+6 {
		t.Fatalf("unexpected run dir suffix length: %s", base)
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
