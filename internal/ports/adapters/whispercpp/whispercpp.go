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
		"-ojf",
		"-of", outPrefix,
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

	var raw whisperJSON
	if err := json.Unmarshal(jb, &raw); err != nil {
		return types.Transcript{}, err
	}
	return raw.toTranscript(), nil
}

type whisperJSON struct {
	Transcription []whisperSeg `json:"transcription"`
}

type whisperSeg struct {
	Offsets struct {
		From int `json:"from"`
		To   int `json:"to"`
	} `json:"offsets"`
	Text   string       `json:"text"`
	Tokens []whisperTok `json:"tokens"`
}

type whisperTok struct {
	Text    string `json:"text"`
	Offsets struct {
		From int `json:"from"`
		To   int `json:"to"`
	} `json:"offsets"`
}

func (w whisperJSON) toTranscript() types.Transcript {
	var tr types.Transcript
	for _, s := range w.Transcription {
		seg := types.Segment{
			Start: msToSec(s.Offsets.From),
			End:   msToSec(s.Offsets.To),
			Text:  strings.TrimSpace(s.Text),
			Words: tokensToWords(s.Tokens),
		}
		tr.Segments = append(tr.Segments, seg)
	}
	return tr
}

func tokensToWords(toks []whisperTok) []types.Word {
	var out []types.Word

	var cur string
	var curFrom, curTo int
	flush := func() {
		w := strings.TrimSpace(cur)
		if w == "" {
			cur = ""
			curFrom, curTo = 0, 0
			return
		}
		out = append(out, types.Word{
			Start: msToSec(curFrom),
			End:   msToSec(curTo),
			Word:  w,
		})
		cur = ""
		curFrom, curTo = 0, 0
	}

	for _, t := range toks {
		raw := t.Text
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		// skip control tokens like "[_BEG_]"
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			continue
		}

		startsNewWord := strings.HasPrefix(raw, " ") || strings.HasPrefix(raw, "\n") || strings.HasPrefix(raw, "\t")
		if startsNewWord && cur != "" {
			flush()
		}
		if cur == "" {
			curFrom = t.Offsets.From
		}
		curTo = t.Offsets.To
		cur += trimmed
	}
	flush()
	return out
}

func msToSec(ms int) float64 {
	return float64(ms) / 1000.0
}
