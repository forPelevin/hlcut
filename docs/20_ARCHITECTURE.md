# Architecture

hlcut uses a ports-and-adapters structure to keep the core logic testable.

## Package layout
- `cmd/hlcut/` — CLI entrypoint
- `internal/cli/` — Cobra commands, flags, env loading
- `internal/pipeline/` — wiring + orchestration config
- `internal/usecase/` — application use case (pure coordination of ports)
- `internal/ports/` — interfaces (VideoTool, ASR, LLMRanker)
- `internal/ports/adapters/` — implementations:
  - `ffmpeg/` — extract audio, render clips, probe duration
  - `whispercpp/` — run whisper.cpp, parse JSON, produce transcript with word timestamps
  - `openrouter/` — call OpenRouter chat completions, parse JSON output
- `internal/domain/` — pure domain logic:
  - `highlights/` — candidate windows + heuristic scores
  - `subtitles/` — ASS renderer (TikTok-style karaoke)
- `internal/itest/` — end-to-end integration tests (real ffmpeg + whisper.cpp + OpenRouter)

## Data flow
1. **Extract audio**: MP4 → WAV (mono, 16k)
2. **ASR**: WAV → transcript (segments + words)
3. **Build candidates**: windows (start/end/text + heuristic scores), constrained by min/max duration
4. **LLM refine**: select distinct non-overlapping highlight clips (bounded by requested max count)
5. **Subtitles (optional)**: transcript slice → `.ass` karaoke when `--burn-subtitles` is enabled
6. **Render**: ffmpeg writes final clips; ASS burn-in is enabled only when `--burn-subtitles` is set
7. **Manifest**: JSON index of outputs (may contain zero clips if no valid highlights)

## Testing strategy
- `make test`: unit tests only (fast, deterministic)
- `make itest`: end-to-end integration (slow; uses internet and OpenRouter)
