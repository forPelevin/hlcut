package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/forPelevin/hlcut/internal/pipeline"
	"github.com/spf13/cobra"
)

func run(cmd *cobra.Command, input string) error {
	outDir, _ := cmd.Flags().GetString("out")
	clipsN, _ := cmd.Flags().GetInt("clips")
	maxSec, _ := cmd.Flags().GetInt("max")

	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		return errors.New("OPENROUTER_API_KEY is required (set it in .env)")
	}

	absIn, err := filepath.Abs(input)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Hour)
	defer cancel()

	cfg := pipeline.Config{
		InputMP4: absIn,
		OutDir:   outDir,
		ClipsN:   clipsN,
		MaxClip:  time.Duration(maxSec) * time.Second,

		FFmpegPath:  "ffmpeg",
		FFprobePath: "ffprobe",

		WhisperBin:   ".cache/bin/whisper.cpp",
		WhisperModel: ".cache/models/ggml-base.bin",

		OpenRouterAPIKey:  apiKey,
		OpenRouterModel:   getenvDefault("OPENROUTER_MODEL", "z-ai/glm-4.5-air:free"),
		OpenRouterBaseURL: getenvDefault("OPENROUTER_BASE_URL", "https://openrouter.ai"),
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("config: %w", err)
	}
	return pipeline.Run(ctx, cfg)
}

func getenvDefault(k, def string) string {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	return v
}
