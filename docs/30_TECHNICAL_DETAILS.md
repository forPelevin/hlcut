# Technical details

## Environment
- `.env` is loaded (gitignored). Required:
  - `OPENROUTER_API_KEY=...`
- Optional:
  - `OPENROUTER_MODEL=z-ai/glm-4.5-air:free`
  - `OPENROUTER_BASE_URL=https://openrouter.ai`
  - `OPENROUTER_ALLOWED_HOSTS=openrouter.ai,api.openrouter.ai`

## Make targets
- `make env_up` — build the dev image (`hlcut-env:local`)
- `make setup` — build whisper.cpp + download `ggml-base.bin` into `./.cache/`
- `make test` — unit tests
- `make itest` — integration tests (requires `.env`)

## whisper.cpp integration
- Pins whisper.cpp to a fixed git ref during setup before build.
- Builds `whisper-cli` from whisper.cpp and copies it to `./.cache/bin/whisper.cpp`
- Downloads base model to `./.cache/models/ggml-base.bin` and verifies SHA256.
- Runs whisper with JSON-full output:
  - `-ojf` (token-level timing)
  - Parses token list into approximate **word timestamps** by grouping tokens into words using whitespace boundaries

## OpenRouter integration
- Uses `/api/v1/chat/completions`
- Validates `OPENROUTER_BASE_URL` before requests:
  - absolute URL with host required
  - `https` required
  - host must match configured allowlist
- Requests JSON schema output; still parses defensively:
  - strips code fences
  - extracts the first JSON object substring
- Post-processing constraints:
  - clip duration must be within configured `minClip..maxClip`
  - clips must be distinct and non-overlapping
  - requested `clips` is an upper bound (result can be smaller)
- If model output is malformed/invalid, selection falls back deterministically to best-scoring valid candidates

## ASS karaoke rendering
- Produces line-packed dialogue events across the full selected clip
- Uses `{\k<centiseconds>}` tags per word
- Escapes ASS special characters (`\`, `{`, `}`)

## ffmpeg rendering
- Burns ASS via `-vf subtitles=<path>` only when `--burn-subtitles` is enabled
- Encodes h264 (libx264) + aac
