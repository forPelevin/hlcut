//go:build integration

package itest

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/forPelevin/hlcut/internal/pipeline"
	"github.com/forPelevin/hlcut/internal/types"
)

func TestE2E(t *testing.T) {
	if os.Getenv("OPENROUTER_API_KEY") == "" {
		t.Fatalf("OPENROUTER_API_KEY is required for itest")
	}

	tmp := t.TempDir()
	in := filepath.Join(tmp, "input.mp4")

	// Generate speech audio via espeak-ng.
	wav := filepath.Join(tmp, "speech.wav")
	text := "Here is the key idea. Step one: do this. Step two: measure results. This is important."
	cmd := exec.Command("espeak-ng", "-w", wav, text)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("espeak-ng failed: %v\n%s", err, string(b))
	}

	// Build a simple mp4 with audio.
	ff := exec.Command("ffmpeg",
		"-y",
		"-f", "lavfi",
		"-i", "color=c=black:s=1280x720:d=15",
		"-i", wav,
		"-shortest",
		"-c:v", "libx264",
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		in,
	)
	if b, err := ff.CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg fixture failed: %v\n%s", err, string(b))
	}

	outDir := filepath.Join(tmp, "out")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}

	cfg := pipeline.Config{
		InputMP4:    in,
		OutDir:      outDir,
		ClipsN:      2,
		MaxClip:     60 * time.Second,
		FFmpegPath:  "ffmpeg",
		FFprobePath: "ffprobe",
		CacheDir:    filepath.Join(tmp, "cache"),

		// go test runs each package with its own working directory, so use absolute paths.
		WhisperBin:   filepath.Join(repoRoot, ".cache", "bin", "whisper.cpp"),
		WhisperModel: filepath.Join(repoRoot, ".cache", "models", "ggml-base.bin"),

		OpenRouterAPIKey:  os.Getenv("OPENROUTER_API_KEY"),
		OpenRouterModel:   os.Getenv("OPENROUTER_MODEL"),
		OpenRouterBaseURL: os.Getenv("OPENROUTER_BASE_URL"),
	}
	if cfg.OpenRouterModel == "" {
		cfg.OpenRouterModel = "z-ai/glm-4.5-air:free"
	}
	if cfg.OpenRouterBaseURL == "" {
		cfg.OpenRouterBaseURL = "https://openrouter.ai"
	}

	if err := pipeline.Run(ctx, cfg); err != nil {
		t.Fatalf("pipeline failed: %v", err)
	}

	manifestPath := filepath.Join(outDir, "manifest.json")
	mb, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var m types.Manifest
	if err := json.Unmarshal(mb, &m); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	if len(m.Clips) != 2 {
		t.Fatalf("expected 2 clips, got %d", len(m.Clips))
	}

	for _, c := range m.Clips {
		mp4 := filepath.Join(outDir, filepath.FromSlash(c.File))
		ass := filepath.Join(outDir, filepath.FromSlash(c.Subs))

		if _, err := os.Stat(mp4); err != nil {
			t.Fatalf("missing clip %s: %v", mp4, err)
		}
		if _, err := os.Stat(ass); err != nil {
			t.Fatalf("missing subs %s: %v", ass, err)
		}

		// Ensure karaoke tags exist (word-highlight MVP requirement).
		ab, err := os.ReadFile(ass)
		if err != nil {
			t.Fatalf("read subs %s: %v", ass, err)
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
		if dur > 60.2 {
			t.Fatalf("expected duration <= 60.2s for %s, got %v", mp4, dur)
		}
	}
}
