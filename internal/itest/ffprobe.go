//go:build integration

package itest

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func probeDurationSeconds(mp4Path string) (float64, error) {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		mp4Path,
	)
	b, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("ffprobe: %w\n%s", err, string(b))
	}
	s := strings.TrimSpace(string(b))
	sec, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("parse duration %q: %w", s, err)
	}
	return sec, nil
}
