package pipeline

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/forPelevin/hlcut/internal/ports"
	"github.com/forPelevin/hlcut/internal/ports/adapters/ffmpeg"
	"github.com/forPelevin/hlcut/internal/ports/adapters/openrouter"
	"github.com/forPelevin/hlcut/internal/ports/adapters/whispercpp"
	"github.com/forPelevin/hlcut/internal/usecase"
)

type Config struct {
	InputMP4 string
	OutDir   string
	ClipsN   int
	MaxClip  time.Duration

	// CacheDir is the base directory for local artifacts (audio, transcripts, etc.).
	// If empty, defaults to ".cache".
	CacheDir string

	FFmpegPath  string
	FFprobePath string

	WhisperBin   string
	WhisperModel string

	OpenRouterAPIKey  string
	OpenRouterModel   string
	OpenRouterBaseURL string
}

func (c Config) Validate() error {
	if c.InputMP4 == "" {
		return errors.New("input is empty")
	}
	if _, err := os.Stat(c.InputMP4); err != nil {
		return fmt.Errorf("stat input: %w", err)
	}
	if c.OutDir == "" {
		c.OutDir = "out"
	}
	if c.ClipsN <= 0 {
		return fmt.Errorf("clips must be > 0")
	}
	if c.MaxClip <= 0 {
		return fmt.Errorf("max clip must be > 0")
	}
	if c.WhisperModel == "" {
		return fmt.Errorf("whisper model path is required")
	}
	return nil
}

func Run(ctx context.Context, cfg Config) error {
	// adapters
	v := ffmpeg.New(cfg.FFmpegPath, cfg.FFprobePath)
	asr := whispercpp.New(cfg.WhisperBin, cfg.WhisperModel)
	llm := openrouter.New(cfg.OpenRouterAPIKey, cfg.OpenRouterModel, cfg.OpenRouterBaseURL)

	deps := usecase.Deps{
		Video: v,
		ASR:   asr,
		LLM:   llm,
	}

	uc := usecase.New(deps)

	jobID := hash(cfg.InputMP4)
	baseCache := cfg.CacheDir
	if baseCache == "" {
		baseCache = ".cache"
	}
	cacheDir := filepath.Join(baseCache, "runs", jobID)
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return err
	}

	outDir := cfg.OutDir
	clipsDir := filepath.Join(outDir, "clips")
	subsDir := filepath.Join(outDir, "subs")
	if err := os.MkdirAll(clipsDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(subsDir, 0o755); err != nil {
		return err
	}

	res, err := uc.Run(ctx, usecase.Input{
		InputMP4: cfg.InputMP4,
		ClipsN:   cfg.ClipsN,
		MaxClip:  cfg.MaxClip,
		CacheDir: cacheDir,
		OutDir:   outDir,
	})
	if err != nil {
		return err
	}

	b, _ := json.MarshalIndent(res.Manifest, "", "  ")
	return os.WriteFile(filepath.Join(outDir, "manifest.json"), b, 0o644)
}

func hash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])[:12]
}

// ensure adapters implement ports
var _ ports.VideoTool = (*ffmpeg.Adapter)(nil)
var _ ports.ASR = (*whispercpp.Adapter)(nil)
var _ ports.LLMRanker = (*openrouter.Adapter)(nil)
