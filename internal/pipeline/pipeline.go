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
	InputMP4 string
	OutDir   string
	ClipsN   int
	// ClipsNSet indicates whether --clips was explicitly provided by the user.
	ClipsNSet     bool
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

const (
	autoClipWindow   = 5 * time.Minute
	autoClipMaxLimit = 80
)

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

	clipsN := cfg.ClipsN
	if !cfg.ClipsNSet {
		videoDur, err := v.ProbeDuration(ctx, cfg.InputMP4)
		if err != nil {
			logf("duration probe failed, keeping clip cap %d: %v", clipsN, err)
		} else {
			clipsN = autoClipCount(clipsN, videoDur)
			if clipsN != cfg.ClipsN {
				logf("auto clips cap from duration: %s => %d", formatDuration(videoDur), clipsN)
			}
		}
	}

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
		ClipsN:        clipsN,
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
	ts := now.UTC().UnixMilli()
	runSeed := fmt.Sprintf("%s|%d", inputMP4, now.UTC().UnixNano())
	suffix := hash(runSeed)[:6]
	return filepath.Join(outRoot, fmt.Sprintf("%013d-%s-%s", ts, name, suffix))
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

func autoClipCount(base int, duration time.Duration) int {
	if base <= 0 || duration <= 0 {
		return base
	}

	estimated := int(duration / autoClipWindow)
	if duration%autoClipWindow != 0 {
		estimated++
	}

	if base >= estimated {
		return base
	}

	clipsN := estimated
	if clipsN > autoClipMaxLimit {
		return autoClipMaxLimit
	}
	return clipsN
}

func formatDuration(d time.Duration) string {
	return d.Round(100 * time.Millisecond).String()
}

// ensure adapters implement ports
var _ ports.VideoTool = (*ffmpeg.Adapter)(nil)
var _ ports.ASR = (*whispercpp.Adapter)(nil)
var _ ports.LLMRanker = (*openrouter.Adapter)(nil)
