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
	"strings"
	"time"
	"unicode"

	"github.com/forPelevin/hlcut/internal/ports"
	"github.com/forPelevin/hlcut/internal/ports/adapters/ffmpeg"
	"github.com/forPelevin/hlcut/internal/ports/adapters/openrouter"
	"github.com/forPelevin/hlcut/internal/ports/adapters/whispercpp"
	"github.com/forPelevin/hlcut/internal/usecase"
)

type Config struct {
	InputMP4      string
	OutDir        string
	ClipsN        int
	MinClip       time.Duration
	MaxClip       time.Duration
	BurnSubtitles bool
	Logf          func(format string, args ...any)

	// CacheDir is the base directory for local artifacts (audio, transcripts, etc.).
	// If empty, defaults to ".cache".
	CacheDir string

	FFmpegPath  string
	FFprobePath string

	WhisperBin   string
	WhisperModel string

	OpenRouterAPIKey       string
	OpenRouterModel        string
	OpenRouterBaseURL      string
	OpenRouterAllowedHosts []string
}

func (c Config) Validate() error {
	if c.InputMP4 == "" {
		return errors.New("input is empty")
	}
	if _, err := os.Stat(c.InputMP4); err != nil {
		return fmt.Errorf("stat input: %w", err)
	}
	if c.ClipsN <= 0 {
		return fmt.Errorf("clips must be > 0")
	}
	if c.MaxClip <= 0 {
		return fmt.Errorf("max clip must be > 0")
	}
	if c.MinClip <= 0 {
		return fmt.Errorf("min clip must be > 0")
	}
	if c.MinClip > c.MaxClip {
		return fmt.Errorf("min clip must be <= max clip")
	}
	if c.WhisperModel == "" {
		return fmt.Errorf("whisper model path is required")
	}
	return openrouter.ValidateBaseURL(
		c.OpenRouterBaseURL,
		c.OpenRouterAllowedHosts,
	)
}

func Run(ctx context.Context, cfg Config) error {
	logf := cfg.Logf
	if logf == nil {
		logf = func(string, ...any) {}
	}

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
	logf("preparing workspace")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return err
	}
	logf("cache: %s", cacheDir)

	outDir := cfg.OutDir
	if outDir == "" {
		outDir = "out"
	}
	runOutDir := buildRunOutDir(outDir, cfg.InputMP4, time.Now().UTC())
	clipsDir := filepath.Join(runOutDir, "clips")
	subtitlesDir := filepath.Join(runOutDir, "subtitles")
	if err := os.MkdirAll(clipsDir, 0o755); err != nil {
		return err
	}
	if cfg.BurnSubtitles {
		if err := os.MkdirAll(subtitlesDir, 0o755); err != nil {
			return err
		}
		logf("output run dir: %s", runOutDir)
		logf("output dirs: %s, %s", clipsDir, subtitlesDir)
	} else {
		logf("output run dir: %s", runOutDir)
		logf("output dirs: %s", clipsDir)
	}

	res, err := uc.Run(ctx, usecase.Input{
		InputMP4:      cfg.InputMP4,
		ClipsN:        cfg.ClipsN,
		MinClip:       cfg.MinClip,
		MaxClip:       cfg.MaxClip,
		BurnSubtitles: cfg.BurnSubtitles,
		CacheDir:      cacheDir,
		OutDir:        runOutDir,
		Logf:          logf,
	})
	if err != nil {
		return err
	}

	b, err := json.MarshalIndent(res.Manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	manifestPath := filepath.Join(runOutDir, "manifest.json")
	if err := os.WriteFile(manifestPath, b, 0o644); err != nil {
		return err
	}
	logf("manifest written (%d clips): %s", len(res.Manifest.Clips), manifestPath)
	return nil
}

func buildRunOutDir(outRoot, inputMP4 string, now time.Time) string {
	name := strings.TrimSuffix(filepath.Base(inputMP4), filepath.Ext(inputMP4))
	name = normalizePathSegment(name)
	if name == "" {
		name = "input"
	}
	ts := now.UTC().Format("20060102-150405Z")
	runSeed := fmt.Sprintf("%s|%d", inputMP4, now.UTC().UnixNano())
	suffix := hash(runSeed)[:6]
	return filepath.Join(outRoot, fmt.Sprintf("%s-%s-%s", name, ts, suffix))
}

func normalizePathSegment(s string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func hash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])[:12]
}

// ensure adapters implement ports
var _ ports.VideoTool = (*ffmpeg.Adapter)(nil)
var _ ports.ASR = (*whispercpp.Adapter)(nil)
var _ ports.LLMRanker = (*openrouter.Adapter)(nil)
