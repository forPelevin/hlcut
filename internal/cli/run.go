package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/forPelevin/hlcut/internal/pipeline"
	"github.com/spf13/cobra"
)

func run(cmd *cobra.Command, input string) error {
	started := time.Now()
	logf := newRunLogger(cmd.ErrOrStderr(), started)

	outDir, err := cmd.Flags().GetString("out")
	if err != nil {
		return fmt.Errorf("read out flag: %w", err)
	}
	clipsN, err := cmd.Flags().GetInt("clips")
	if err != nil {
		return fmt.Errorf("read clips flag: %w", err)
	}
	maxSec, err := cmd.Flags().GetInt("max")
	if err != nil {
		return fmt.Errorf("read max flag: %w", err)
	}
	minSec, err := cmd.Flags().GetInt("min")
	if err != nil {
		return fmt.Errorf("read min flag: %w", err)
	}
	burnSubtitles, err := cmd.Flags().GetBool("burn-subtitles")
	if err != nil {
		return fmt.Errorf("read burn-subtitles flag: %w", err)
	}

	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		return errors.New("OPENROUTER_API_KEY is required (set it in .env)")
	}

	absIn, err := filepath.Abs(input)
	if err != nil {
		return err
	}
	absOut, err := filepath.Abs(outDir)
	if err != nil {
		return err
	}

	logf("starting run")
	logf("input: %s", absIn)
	logf("output: %s", absOut)
	logf("requested clips: %d (%d-%ds each)", clipsN, minSec, maxSec)
	logf("burn subtitles: %t", burnSubtitles)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Hour)
	defer cancel()

	cfg := pipeline.Config{
		InputMP4:      absIn,
		OutDir:        outDir,
		ClipsN:        clipsN,
		MinClip:       time.Duration(minSec) * time.Second,
		MaxClip:       time.Duration(maxSec) * time.Second,
		BurnSubtitles: burnSubtitles,

		FFmpegPath:  "ffmpeg",
		FFprobePath: "ffprobe",

		CacheDir: ".cache",

		WhisperBin:   ".cache/bin/whisper.cpp",
		WhisperModel: ".cache/models/ggml-base.bin",

		OpenRouterAPIKey:  apiKey,
		OpenRouterModel:   getenvDefault("OPENROUTER_MODEL", "z-ai/glm-4.5-air:free"),
		OpenRouterBaseURL: getenvDefault("OPENROUTER_BASE_URL", "https://openrouter.ai"),
		OpenRouterAllowedHosts: getenvCSV(
			"OPENROUTER_ALLOWED_HOSTS",
			"openrouter.ai,api.openrouter.ai",
		),
		Logf: logf,
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("config: %w", err)
	}
	if err := pipeline.Run(ctx, cfg); err != nil {
		logf("run failed after %s: %v", shortDuration(time.Since(started)), err)
		return err
	}

	logf("run completed in %s", shortDuration(time.Since(started)))
	return nil
}

func getenvDefault(k, def string) string {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	return v
}

func getenvCSV(k, def string) []string {
	raw := strings.TrimSpace(os.Getenv(k))
	if raw == "" {
		raw = def
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

func newRunLogger(out io.Writer, started time.Time) func(format string, args ...any) {
	l := &runLogger{
		out:     out,
		started: started,
	}
	if os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb" {
		if f, ok := out.(*os.File); ok {
			if info, err := f.Stat(); err == nil && (info.Mode()&os.ModeCharDevice) != 0 {
				l.useANSI = true
			}
		}
	}
	return l.logf
}

func shortDuration(d time.Duration) string {
	return d.Round(100 * time.Millisecond).String()
}

var (
	stageStartRE = regexp.MustCompile(`^stage (\d+)/(\d+): (.+)$`)
	stageDoneRE  = regexp.MustCompile(`^stage (\d+)/(\d+) done in (.+)$`)
	clipRE       = regexp.MustCompile(`^rendering clip (\d+)/(\d+) \(([^)]+)\) \[(.+) -> (.+)\]$`)
)

type runLogger struct {
	out         io.Writer
	started     time.Time
	useANSI     bool
	headerShown bool
}

func (l *runLogger) logf(format string, args ...any) {
	if l.out == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)

	if !l.headerShown {
		l.headerShown = true
		l.printRaw(l.color(cBold, "┏━ 🚀 hlcut run ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"))
	}

	icon, body, color := l.decorate(msg)
	timeLabel := "⏱ " + shortDuration(time.Since(l.started))
	line := fmt.Sprintf(
		"%s %s %s %s",
		l.color(cCyan, timeLabel),
		l.color(cDim, "│"),
		l.color(color, icon),
		l.color(color, body),
	)
	l.printRaw(line)

	if strings.HasPrefix(msg, "run completed") || strings.HasPrefix(msg, "run failed") {
		l.printRaw(l.color(cBold, "┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"))
	}
}

func (l *runLogger) decorate(msg string) (icon string, body string, color string) {
	if msg == "starting run" {
		return "🚀", "Starting pipeline", cBlue
	}
	if strings.HasPrefix(msg, "input: ") {
		return "🎬", "Input  " + strings.TrimPrefix(msg, "input: "), cCyan
	}
	if strings.HasPrefix(msg, "output: ") {
		return "📁", "Output " + strings.TrimPrefix(msg, "output: "), cCyan
	}
	if strings.HasPrefix(msg, "requested clips: ") {
		return "✂️", "Plan   " + strings.TrimPrefix(msg, "requested clips: "), cYellow
	}
	if strings.HasPrefix(msg, "preparing workspace") {
		return "🧱", "Preparing workspace", cBlue
	}
	if strings.HasPrefix(msg, "cache: ") {
		return "🗄️", "Cache  " + strings.TrimPrefix(msg, "cache: "), cBlue
	}
	if strings.HasPrefix(msg, "output dirs: ") {
		return "📂", "Dirs   " + strings.TrimPrefix(msg, "output dirs: "), cBlue
	}
	if strings.HasPrefix(msg, "manifest written") {
		return "🧾", msg, cGreen
	}
	if strings.HasPrefix(msg, "manifest: ") {
		return "📌", msg, cGreen
	}
	if strings.HasPrefix(msg, "run completed") {
		return "✅", msg, cGreen
	}
	if strings.HasPrefix(msg, "run failed") {
		return "💥", msg, cRed
	}

	if m := stageStartRE.FindStringSubmatch(msg); len(m) == 4 {
		done, total := parseInt(m[1]), parseInt(m[2])
		return "🛠️", fmt.Sprintf("Stage %d/%d [%s] %s", done, total, progressBar(done-1, total, 10), m[3]), cMagenta
	}
	if m := stageDoneRE.FindStringSubmatch(msg); len(m) == 4 {
		done, total := parseInt(m[1]), parseInt(m[2])
		return "✅", fmt.Sprintf("Stage %d/%d [%s] done in %s", done, total, progressBar(done, total, 10), m[3]), cGreen
	}
	if m := clipRE.FindStringSubmatch(msg); len(m) == 6 {
		i, total := parseInt(m[1]), parseInt(m[2])
		return "🎞️", fmt.Sprintf("Clip %s %d/%d [%s] %s -> %s", m[3], i, total, progressBar(i, total, 10), m[4], m[5]), cYellow
	}

	return "•", msg, cDim
}

func parseInt(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

func progressBar(done, total, width int) string {
	if width <= 0 {
		return ""
	}
	if total <= 0 {
		return strings.Repeat("░", width)
	}
	if done < 0 {
		done = 0
	}
	if done > total {
		done = total
	}
	filled := done * width / total
	if done > 0 && filled == 0 {
		filled = 1
	}
	if done == total {
		filled = width
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

func (l *runLogger) printRaw(line string) {
	fmt.Fprintln(l.out, line)
}

func (l *runLogger) color(code string, s string) string {
	if !l.useANSI || code == "" {
		return s
	}
	return code + s + cReset
}

const (
	cReset   = "\033[0m"
	cBold    = "\033[1m"
	cDim     = "\033[90m"
	cRed     = "\033[31m"
	cGreen   = "\033[32m"
	cYellow  = "\033[33m"
	cBlue    = "\033[34m"
	cMagenta = "\033[35m"
	cCyan    = "\033[36m"
)
