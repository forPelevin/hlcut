package ffmpeg

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type Adapter struct {
	ffmpeg  string
	ffprobe string
}

func New(ffmpegPath, ffprobePath string) *Adapter {
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}
	if ffprobePath == "" {
		ffprobePath = "ffprobe"
	}
	return &Adapter{ffmpeg: ffmpegPath, ffprobe: ffprobePath}
}

func (a *Adapter) ExtractAudioMono16k(ctx context.Context, inMP4, outWav string) error {
	cmd := exec.CommandContext(ctx, a.ffmpeg,
		"-y",
		"-i", inMP4,
		"-vn",
		"-ac", "1",
		"-ar", "16000",
		"-f", "wav",
		outWav,
	)
	b, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg extract audio: %w\n%s", err, string(b))
	}
	return nil
}

func (a *Adapter) RenderClip(ctx context.Context, inMP4 string, start, end time.Duration, outMP4 string, burnASS string) error {
	args := []string{
		"-y",
		"-ss", fmtSeconds(start),
		"-to", fmtSeconds(end),
		"-i", inMP4,
	}
	if burnASS != "" {
		args = append(args, "-vf", "subtitles="+escapeFilterPath(burnASS))
	}
	args = append(args,
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-crf", "18",
		"-c:a", "aac",
		"-b:a", "192k",
		outMP4,
	)
	cmd := exec.CommandContext(ctx, a.ffmpeg, args...)
	b, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg render clip: %w\n%s", err, string(b))
	}
	return nil
}

func (a *Adapter) ProbeDuration(ctx context.Context, inMP4 string) (time.Duration, error) {
	cmd := exec.CommandContext(ctx, a.ffprobe,
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		inMP4,
	)
	b, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("ffprobe duration: %w\n%s", err, string(b))
	}
	s := strings.TrimSpace(string(b))
	sec, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("parse duration %q: %w", s, err)
	}
	return time.Duration(sec * float64(time.Second)), nil
}

func fmtSeconds(d time.Duration) string {
	sec := float64(d) / float64(time.Second)
	return strconv.FormatFloat(sec, 'f', 3, 64)
}

func escapeFilterPath(p string) string {
	p = strings.ReplaceAll(p, "\\", "\\\\")
	p = strings.ReplaceAll(p, ":", "\\:")
	return p
}
