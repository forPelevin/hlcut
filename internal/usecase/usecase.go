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
	InputMP4 string
	ClipsN   int
	MaxClip  time.Duration
	CacheDir string
	OutDir   string
}

type Result struct {
	Manifest types.Manifest
}

func (u Usecase) Run(ctx context.Context, in Input) (Result, error) {
	wav := filepath.Join(in.CacheDir, "audio.wav")
	if err := u.d.Video.ExtractAudioMono16k(ctx, in.InputMP4, wav); err != nil {
		return Result{}, err
	}

	tr, err := u.d.ASR.Transcribe(ctx, wav, in.CacheDir)
	if err != nil {
		return Result{}, err
	}

	cands := highlights.BuildCandidates(tr, in.MaxClip)
	clipSpecs, err := u.d.LLM.Refine(ctx, tr, cands, in.ClipsN, in.MaxClip)
	if err != nil {
		return Result{}, err
	}

	m := types.Manifest{Input: in.InputMP4}
	for i, cs := range clipSpecs {
		id := fmt.Sprintf("%03d", i+1)
		clipPath := filepath.Join(in.OutDir, "clips", id+".mp4")
		assPath := filepath.Join(in.OutDir, "subs", id+".ass")

		// subs
		ass, err := subtitles.RenderTikTokASS(tr, cs.Start, cs.End)
		if err != nil {
			return Result{}, err
		}
		if err := writeFile(assPath, []byte(ass)); err != nil {
			return Result{}, err
		}

		// render
		if err := u.d.Video.RenderClip(ctx, in.InputMP4, cs.Start, cs.End, clipPath, assPath); err != nil {
			return Result{}, err
		}

		// NOTE: candidate text lookup is TODO; store empty for now
		m.Clips = append(m.Clips, types.ManifestClip{
			ID:        id,
			StartSec:  cs.Start.Seconds(),
			EndSec:    cs.End.Seconds(),
			File:      filepath.ToSlash(filepath.Join("clips", id+".mp4")),
			Subs:      filepath.ToSlash(filepath.Join("subs", id+".ass")),
			Title:     cs.Title,
			Caption:   cs.Caption,
			Tags:      cs.Tags,
			InfoScore: 0,
			HookScore: 0,
			Text:      "",
		})
	}

	return Result{Manifest: m}, nil
}

func writeFile(path string, b []byte) error {
	return os.WriteFile(path, b, 0o644)
}
