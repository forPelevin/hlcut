# Features

## Current
- **One-command CLI**: `hlcut <input.mp4>`
- **Dockerized dev/test environment** (no host dependencies required beyond Docker)
- **Local whisper.cpp build + model download** via `make setup` into `./.cache/` (gitignored)
- **ASS karaoke subtitles** (optional via `--burn-subtitles`):
  - Word-level highlight using `Dialogue` lines with `{\k<centisec>}` tags
  - Fallback to plain ASS when word timestamps aren’t available
  - Subtitle events span the full selected clip text (no fixed 2-line truncation)
- **Highlight candidate generation**:
  - Prefer word-timestamp windows (more granular)
  - Fallback to segment windows
  - Enforces duration bounds: `minClip..maxClip` (defaults: 20..60s)
  - Samples candidates across the full transcript timeline
- **LLM ranking/refinement** via OpenRouter:
  - Sends a bounded candidate list
  - Requests strict JSON (schema)
  - Robust parsing (strips code fences / extracts first JSON object)
  - Enforces distinct non-overlapping clips and duration bounds
  - `--clips` is treated as an upper bound (can return fewer clips)
  - Deterministic fallback selection if model output is malformed/invalid
- **Output hygiene**:
  - each run gets a fresh output subdirectory under `--out` (no destructive cleanup of previous runs)

## Planned next
- Better semantic topic diversity
- Better clip boundaries (sentence-aware)
- Multiple subtitle styles / safe fonts
- URL input support (download + caching)
- Config file support (still keep CLI stable)
