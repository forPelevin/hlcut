package whispercpp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/forPelevin/hlcut/internal/types"
)

type Adapter struct {
	bin   string
	model string
}

func New(binPath, modelPath string) *Adapter {
	return &Adapter{bin: binPath, model: modelPath}
}

func (a *Adapter) Transcribe(ctx context.Context, wavPath, cacheDir string) (types.Transcript, error) {
	outPrefix := filepath.Join(cacheDir, "whisper")
	args := []string{
		"-m", a.model,
		"-f", wavPath,
		"-oj",
		"-of", outPrefix,
		"-owts",
	}
	cmd := exec.CommandContext(ctx, a.bin, args...)
	b, err := cmd.CombinedOutput()
	if err != nil {
		return types.Transcript{}, fmt.Errorf("whisper.cpp failed: %w\n%s", err, string(b))
	}

	jb, err := os.ReadFile(outPrefix + ".json")
	if err != nil {
		return types.Transcript{}, err
	}

	var tr types.Transcript
	if err := json.Unmarshal(jb, &tr); err != nil {
		return types.Transcript{}, err
	}
	for i := range tr.Segments {
		tr.Segments[i].Text = strings.TrimSpace(tr.Segments[i].Text)
		for j := range tr.Segments[i].Words {
			tr.Segments[i].Words[j].Word = strings.TrimSpace(tr.Segments[i].Words[j].Word)
		}
	}
	return tr, nil
}
