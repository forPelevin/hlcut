package usecase

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/forPelevin/hlcut/internal/types"
)

func TestRun_BurnSubtitlesToggle(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		burnSubtitles bool
	}{
		{name: "disabled", burnSubtitles: false},
		{name: "enabled", burnSubtitles: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tmp := t.TempDir()
			outDir := filepath.Join(tmp, "out")
			clipsDir := filepath.Join(outDir, "clips")
			subtitlesDir := filepath.Join(outDir, "subtitles")
			if err := os.MkdirAll(clipsDir, 0o755); err != nil {
				t.Fatalf("mkdir clips dir: %v", err)
			}
			if err := os.MkdirAll(subtitlesDir, 0o755); err != nil {
				t.Fatalf("mkdir subtitles dir: %v", err)
			}

			video := &fakeVideoTool{}
			uc := New(Deps{
				Video: video,
				ASR:   fakeASR{tr: testTranscript()},
				LLM: fakeLLM{clips: []types.ClipSpec{
					{
						Start:   0,
						End:     5 * time.Second,
						Title:   "t",
						Caption: "c",
						Tags:    []string{"x"},
					},
				}},
			})

			res, err := uc.Run(context.Background(), Input{
				InputMP4:      filepath.Join(tmp, "in.mp4"),
				ClipsN:        1,
				MinClip:       5 * time.Second,
				MaxClip:       10 * time.Second,
				BurnSubtitles: tc.burnSubtitles,
				CacheDir:      filepath.Join(tmp, "cache"),
				OutDir:        outDir,
			})
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			if len(video.renderBurnASS) != 1 {
				t.Fatalf("expected 1 rendered clip, got %d", len(video.renderBurnASS))
			}
			if len(res.Manifest.Clips) != 1 {
				t.Fatalf("expected 1 clip in manifest, got %d", len(res.Manifest.Clips))
			}

			subtitlesPath := filepath.Join(subtitlesDir, "001.ass")
			manifestSubtitles := res.Manifest.Clips[0].Subtitles
			if tc.burnSubtitles {
				if video.renderBurnASS[0] == "" {
					t.Fatalf("expected burnASS path to be passed to renderer")
				}
				if !strings.HasSuffix(video.renderBurnASS[0], filepath.Join("subtitles", "001.ass")) {
					t.Fatalf("unexpected burnASS path: %s", video.renderBurnASS[0])
				}
				if manifestSubtitles != filepath.ToSlash(filepath.Join("subtitles", "001.ass")) {
					t.Fatalf("unexpected manifest subtitles path: %q", manifestSubtitles)
				}
				b, err := os.ReadFile(subtitlesPath)
				if err != nil {
					t.Fatalf("read subtitles: %v", err)
				}
				if !strings.Contains(string(b), "{\\k") {
					t.Fatalf("expected karaoke tags in generated subtitles")
				}
				return
			}

			if video.renderBurnASS[0] != "" {
				t.Fatalf("expected empty burnASS path, got %q", video.renderBurnASS[0])
			}
			if manifestSubtitles != "" {
				t.Fatalf("expected empty manifest subtitles path, got %q", manifestSubtitles)
			}
			if _, err := os.Stat(subtitlesPath); !os.IsNotExist(err) {
				t.Fatalf("expected no subtitle file, stat err=%v", err)
			}
		})
	}
}

type fakeVideoTool struct {
	renderBurnASS []string
	renderStarts  []time.Duration
}

func (f *fakeVideoTool) ExtractAudioMono16k(_ context.Context, _, _ string) error {
	return nil
}

func (f *fakeVideoTool) RenderClip(
	_ context.Context,
	_ string,
	start time.Duration,
	_ time.Duration,
	_ string,
	burnASS string,
) error {
	f.renderBurnASS = append(f.renderBurnASS, burnASS)
	f.renderStarts = append(f.renderStarts, start)
	return nil
}

func (f *fakeVideoTool) ProbeDuration(_ context.Context, _ string) (time.Duration, error) {
	return 0, nil
}

type fakeASR struct {
	tr types.Transcript
}

func (f fakeASR) Transcribe(_ context.Context, _, _ string) (types.Transcript, error) {
	return f.tr, nil
}

type fakeLLM struct {
	clips []types.ClipSpec
}

func (f fakeLLM) Refine(
	_ context.Context,
	_ types.Transcript,
	_ []types.Candidate,
	_ int,
	_ time.Duration,
	_ time.Duration,
) ([]types.ClipSpec, error) {
	return f.clips, nil
}

func testTranscript() types.Transcript {
	return types.Transcript{
		Segments: []types.Segment{
			{
				Start: 0,
				End:   5,
				Text:  "hello world",
				Words: []types.Word{
					{Start: 0.1, End: 0.7, Word: "hello"},
					{Start: 0.8, End: 1.4, Word: "world"},
				},
			},
		},
	}
}

func TestRun_SortsClipsByTimeline(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	outDir := filepath.Join(tmp, "out")
	clipsDir := filepath.Join(outDir, "clips")
	if err := os.MkdirAll(clipsDir, 0o755); err != nil {
		t.Fatalf("mkdir clips dir: %v", err)
	}

	video := &fakeVideoTool{}
	uc := New(Deps{
		Video: video,
		ASR:   fakeASR{tr: testTranscript()},
		LLM: fakeLLM{clips: []types.ClipSpec{
			{
				Start:   2 * time.Minute,
				End:     2*time.Minute + 20*time.Second,
				Title:   "late",
				Caption: "late",
			},
			{
				Start:   20 * time.Second,
				End:     40 * time.Second,
				Title:   "early",
				Caption: "early",
			},
		}},
	})

	res, err := uc.Run(context.Background(), Input{
		InputMP4: filepath.Join(tmp, "in.mp4"),
		ClipsN:   2,
		MinClip:  20 * time.Second,
		MaxClip:  3 * time.Minute,
		CacheDir: filepath.Join(tmp, "cache"),
		OutDir:   outDir,
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if len(res.Manifest.Clips) != 2 {
		t.Fatalf("expected 2 clips in manifest, got %d", len(res.Manifest.Clips))
	}
	if res.Manifest.Clips[0].StartSec > res.Manifest.Clips[1].StartSec {
		t.Fatalf("expected clips sorted by start time, got %.2f then %.2f", res.Manifest.Clips[0].StartSec, res.Manifest.Clips[1].StartSec)
	}
	if res.Manifest.Clips[0].ID != "001" || res.Manifest.Clips[1].ID != "002" {
		t.Fatalf("expected sequential ids for sorted clips, got %s and %s", res.Manifest.Clips[0].ID, res.Manifest.Clips[1].ID)
	}
	if len(video.renderStarts) != 2 {
		t.Fatalf("expected 2 render calls, got %d", len(video.renderStarts))
	}
	if video.renderStarts[0] > video.renderStarts[1] {
		t.Fatalf("expected render order to follow timeline, got %s then %s", video.renderStarts[0], video.renderStarts[1])
	}
}
