//go:build integration

package itest

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/forPelevin/hlcut/internal/types"
)

func TestE2E(t *testing.T) {
	if os.Getenv("OPENROUTER_API_KEY") == "" {
		t.Fatalf("OPENROUTER_API_KEY is required for itest")
	}

	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}

	sample := filepath.Join(repoRoot, "internal", "itest", "testdata", "podcast_short.mp4")
	if _, err := os.Stat(sample); err != nil {
		t.Fatalf("missing fixture %s: %v", sample, err)
	}
	whisperBin := filepath.Join(repoRoot, ".cache", "bin", "whisper.cpp")
	if _, err := os.Stat(whisperBin); err != nil {
		t.Fatalf("missing whisper binary %s: %v (run make setup first)", whisperBin, err)
	}
	whisperModel := filepath.Join(repoRoot, ".cache", "models", "ggml-base.bin")
	if _, err := os.Stat(whisperModel); err != nil {
		t.Fatalf("missing whisper model %s: %v (run make setup first)", whisperModel, err)
	}

	tmp := t.TempDir()
	outDir := filepath.Join(tmp, "out")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		"go", "run", "./cmd/hlcut",
		sample,
		"--out", outDir,
		"--clips", "2",
		"--burn-subtitles",
		"--min", "5",
		"--max", "60",
	)
	cmd.Dir = repoRoot
	cmd.Env = os.Environ()
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("cli failed: %v\n%s", err, string(b))
	}

	runDirs, err := filepath.Glob(filepath.Join(outDir, "*"))
	if err != nil {
		t.Fatalf("glob run dirs: %v", err)
	}
	if len(runDirs) != 1 {
		t.Fatalf("expected exactly one run output dir, got %d (%v)", len(runDirs), runDirs)
	}
	runOutDir := runDirs[0]

	manifestPath := filepath.Join(runOutDir, "manifest.json")
	mb, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var m types.Manifest
	if err := json.Unmarshal(mb, &m); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	if len(m.Clips) == 0 {
		t.Fatalf("expected at least 1 clip, got 0")
	}
	if len(m.Clips) > 2 {
		t.Fatalf("expected at most 2 clips, got %d", len(m.Clips))
	}

	clips := make([]types.ManifestClip, len(m.Clips))
	copy(clips, m.Clips)
	sort.Slice(clips, func(i, j int) bool {
		return clips[i].StartSec < clips[j].StartSec
	})

	var prevEnd float64
	for i, c := range clips {
		if c.EndSec-c.StartSec < 5 {
			t.Fatalf("expected duration >= 5s for clip %s, got %.2fs", c.ID, c.EndSec-c.StartSec)
		}
		if i > 0 && c.StartSec < prevEnd {
			t.Fatalf("clips overlap: prev end %.2f, current start %.2f", prevEnd, c.StartSec)
		}
		prevEnd = c.EndSec
	}

	for _, c := range m.Clips {
		mp4 := filepath.Join(runOutDir, filepath.FromSlash(c.File))
		ass := filepath.Join(runOutDir, filepath.FromSlash(c.Subtitles))

		if _, err := os.Stat(mp4); err != nil {
			t.Fatalf("missing clip %s: %v", mp4, err)
		}
		if _, err := os.Stat(ass); err != nil {
			t.Fatalf("missing subtitles %s: %v", ass, err)
		}

		// Ensure karaoke tags exist (word-highlight MVP requirement).
		ab, err := os.ReadFile(ass)
		if err != nil {
			t.Fatalf("read subtitles %s: %v", ass, err)
		}
		if !strings.Contains(string(ab), "{\\k") {
			t.Fatalf("expected karaoke tags in %s", ass)
		}

		dur, err := probeDurationSeconds(mp4)
		if err != nil {
			t.Fatalf("probe duration %s: %v", mp4, err)
		}
		if dur <= 0 {
			t.Fatalf("expected positive duration for %s, got %v", mp4, dur)
		}
		if dur < 4.8 {
			t.Fatalf("expected duration >= 4.8s for %s, got %v", mp4, dur)
		}
		if dur > 60.2 {
			t.Fatalf("expected duration <= 60.2s for %s, got %v", mp4, dur)
		}
	}
}
