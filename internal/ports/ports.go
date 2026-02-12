package ports

import (
	"context"
	"time"

	"github.com/forPelevin/hlcut/internal/types"
)

type VideoTool interface {
	ExtractAudioMono16k(ctx context.Context, inMP4, outWav string) error
	RenderClip(ctx context.Context, inMP4 string, start, end time.Duration, outMP4 string, burnASS string) error
	ProbeDuration(ctx context.Context, inMP4 string) (time.Duration, error)
}

type ASR interface {
	Transcribe(ctx context.Context, wavPath, cacheDir string) (types.Transcript, error)
}

type LLMRanker interface {
	Refine(
		ctx context.Context,
		tr types.Transcript,
		cands []types.Candidate,
		clipsN int,
		minClip time.Duration,
		maxClip time.Duration,
	) ([]types.ClipSpec, error)
}
