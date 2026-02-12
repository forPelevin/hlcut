package usecase

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/forPelevin/hlcut/internal/domain/highlights"
	"github.com/forPelevin/hlcut/internal/domain/subtitles"
	"github.com/forPelevin/hlcut/internal/ports"
	"github.com/forPelevin/hlcut/internal/types"
)

type Deps struct {
	Video ports.VideoTool
	ASR   ports.ASR
	LLM   ports.LLMRanker
}

type Usecase struct{ d Deps }

func New(d Deps) Usecase { return Usecase{d: d} }

type Input struct {
	InputMP4      string
	ClipsN        int
	MinClip       time.Duration
	MaxClip       time.Duration
	BurnSubtitles bool
	CacheDir      string
	OutDir        string
	Logf          func(format string, args ...any)
}

type Result struct {
	Manifest types.Manifest
}

func (u Usecase) Run(ctx context.Context, in Input) (Result, error) {
	// Stages are ordered and fail-fast on purpose: each stage consumes artifacts
	// from the previous one, so continuing after an error would only produce
	// misleading partial output.
	wav := filepath.Join(in.CacheDir, "audio.wav")

	logf(in.Logf, "stage 1/5: extracting audio")
	stageStart := time.Now()
	if err := u.d.Video.ExtractAudioMono16k(ctx, in.InputMP4, wav); err != nil {
		return Result{}, err
	}
	logf(in.Logf, "stage 1/5 done in %s", shortDuration(time.Since(stageStart)))

	logf(in.Logf, "stage 2/5: transcribing audio")
	stageStart = time.Now()
	tr, err := u.d.ASR.Transcribe(ctx, wav, in.CacheDir)
	if err != nil {
		return Result{}, err
	}
	logf(
		in.Logf,
		"stage 2/5 done in %s (%d segments, %d words)",
		shortDuration(time.Since(stageStart)),
		len(tr.Segments),
		countWords(tr),
	)

	logf(in.Logf, "stage 3/5: generating candidate windows")
	stageStart = time.Now()
	// Candidate generation is intentionally broad; final selection constraints
	// (quality, non-overlap, count) are enforced in the LLM refinement stage.
	cands := highlights.BuildCandidates(tr, in.MinClip, in.MaxClip)
	logf(in.Logf, "stage 3/5 done in %s (%d candidates)", shortDuration(time.Since(stageStart)), len(cands))

	logf(in.Logf, "stage 4/5: refining clips with llm")
	stageStart = time.Now()
	clipSpecs, err := u.d.LLM.Refine(ctx, tr, cands, in.ClipsN, in.MinClip, in.MaxClip)
	if err != nil {
		return Result{}, err
	}
	logf(in.Logf, "stage 4/5 done in %s (%d selected)", shortDuration(time.Since(stageStart)), len(clipSpecs))
	if len(clipSpecs) == 0 {
		logf(
			in.Logf,
			"no highlights found (%s to %s, distinct non-overlapping windows); writing empty manifest",
			formatTimestamp(in.MinClip),
			formatTimestamp(in.MaxClip),
		)
	}

	if in.BurnSubtitles {
		logf(in.Logf, "stage 5/5: rendering clips and subtitles")
	} else {
		logf(in.Logf, "stage 5/5: rendering clips")
	}
	stageStart = time.Now()
	m := types.Manifest{Input: in.InputMP4}
	for i, cs := range clipSpecs {
		id := fmt.Sprintf("%03d", i+1)
		clipPath := filepath.Join(in.OutDir, "clips", id+".mp4")
		assPath := ""
		subtitlesPath := ""
		logf(
			in.Logf,
			"rendering clip %d/%d (%s) [%s -> %s]",
			i+1,
			len(clipSpecs),
			id,
			formatTimestamp(cs.Start),
			formatTimestamp(cs.End),
		)

		if in.BurnSubtitles {
			// ASS is rendered as a side artifact before video rendering so ffmpeg can
			// burn the exact subtitle file used for this clip.
			assPath = filepath.Join(in.OutDir, "subtitles", id+".ass")
			ass, err := subtitles.RenderTikTokASS(tr, cs.Start, cs.End)
			if err != nil {
				return Result{}, err
			}
			if err := writeFile(assPath, []byte(ass)); err != nil {
				return Result{}, err
			}
			subtitlesPath = filepath.ToSlash(filepath.Join("subtitles", id+".ass"))
		}

		// render
		if err := u.d.Video.RenderClip(ctx, in.InputMP4, cs.Start, cs.End, clipPath, assPath); err != nil {
			return Result{}, err
		}

		// Manifest shape is kept stable for downstream tools even though candidate
		// text/scores are not wired through from LLM output yet.
		m.Clips = append(m.Clips, types.ManifestClip{
			ID:        id,
			StartSec:  cs.Start.Seconds(),
			EndSec:    cs.End.Seconds(),
			InfoScore: 0,
			HookScore: 0,
			Text:      "",
			File:      filepath.ToSlash(filepath.Join("clips", id+".mp4")),
			Subtitles: subtitlesPath,
			Title:     cs.Title,
			Caption:   cs.Caption,
			Tags:      cs.Tags,
		})
	}
	logf(in.Logf, "stage 5/5 done in %s", shortDuration(time.Since(stageStart)))

	return Result{Manifest: m}, nil
}

func writeFile(path string, b []byte) error {
	return os.WriteFile(path, b, 0o644)
}

func logf(fn func(string, ...any), format string, args ...any) {
	if fn == nil {
		return
	}
	fn(format, args...)
}

func shortDuration(d time.Duration) string {
	return d.Round(100 * time.Millisecond).String()
}

func countWords(tr types.Transcript) int {
	total := 0
	for _, seg := range tr.Segments {
		total += len(seg.Words)
	}
	return total
}

func formatTimestamp(d time.Duration) string {
	total := int(d.Seconds())
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}
