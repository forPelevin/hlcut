# Technical details

## Environment
- `.env` is loaded (gitignored). Required:
  - `OPENROUTER_API_KEY=...`
- Optional:
  - `OPENROUTER_MODEL=z-ai/glm-4.5-air:free`
  - `OPENROUTER_BASE_URL=https://openrouter.ai`

## Make targets
- `make env_up` — build the dev image (`hlcut-env:local`)
- `make setup` — build whisper.cpp + download `ggml-base.bin` into `./.cache/`
- `make test` — unit tests
- `make itest` — integration tests (requires `.env`)

## whisper.cpp integration
- Builds `whisper-cli` from whisper.cpp and copies it to `./.cache/bin/whisper.cpp`
- Downloads base model to `./.cache/models/ggml-base.bin`
- Runs whisper with JSON-full output:
  - `-ojf` (token-level timing)
  - Parses token list into approximate **word timestamps** by grouping tokens into words using whitespace boundaries

## OpenRouter integration
- Uses `/api/v1/chat/completions`
- Requests JSON schema output; still parses defensively:
  - strips code fences
  - extracts the first JSON object substring
- If the model returns invalid timing or too few clips:
  - falls back to candidate boundaries by `idx`
  - pads missing clips from best-scoring candidates

## ASS karaoke rendering
- Produces a small number of on-screen lines (currently up to 2 lines per window)
- Uses `{\k<centiseconds>}` tags per word
- Escapes ASS special characters (`\`, `{`, `}`)

## ffmpeg rendering
- Burns ASS via `-vf subtitles=<path>`
- Encodes h264 (libx264) + aac
