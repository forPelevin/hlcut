# Features

## Current
- **One-command CLI**: `hlcut <input.mp4>`
- **Dockerized dev/test environment** (no host dependencies required beyond Docker)
- **Local whisper.cpp build + model download** via `make setup` into `./.cache/` (gitignored)
- **ASS karaoke subtitles**:
  - Word-level highlight using `Dialogue` lines with `{\k<centisec>}` tags
  - Fallback to plain ASS when word timestamps aren’t available
- **Highlight candidate generation**:
  - Prefer word-timestamp windows (more granular)
  - Fallback to segment windows
  - Hard cap: `maxClip` (default 60s)
- **LLM ranking/refinement** via OpenRouter:
  - Sends a bounded candidate list
  - Requests strict JSON (schema)
  - Robust parsing (strips code fences / extracts first JSON object)
  - Pads results deterministically if the model returns fewer clips than requested

## Planned next
- Better candidate dedup + topic diversity
- Better clip boundaries (sentence-aware)
- Multiple subtitle styles / safe fonts
- URL input support (download + caching)
- Config file support (still keep CLI stable)
