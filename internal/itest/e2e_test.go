//go:build integration

package itest

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/forPelevin/hlcut/internal/pipeline"
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

	cfg := pipeline.Config{
		InputMP4:          in,
		OutDir:            outDir,
		ClipsN:            2,
		MaxClip:           60 * time.Second,
		FFmpegPath:        "ffmpeg",
		FFprobePath:       "ffprobe",
		WhisperBin:        ".cache/bin/whisper.cpp",
		WhisperModel:      ".cache/models/ggml-base.bin",
		OpenRouterAPIKey:  os.Getenv("OPENROUTER_API_KEY"),
		OpenRouterModel:   os.Getenv("OPENROUTER_MODEL"),
		OpenRouterBaseURL: os.Getenv("OPENROUTER_BASE_URL"),
	}
	if cfg.OpenRouterModel == "" {
		cfg.OpenRouterModel = "anthropic/claude-3.5-sonnet"
	}
	if cfg.OpenRouterBaseURL == "" {
		cfg.OpenRouterBaseURL = "https://openrouter.ai"
	}

	if err := pipeline.Run(ctx, cfg); err != nil {
		t.Fatalf("pipeline failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(outDir, "manifest.json")); err != nil {
		t.Fatalf("missing manifest: %v", err)
	}
}
