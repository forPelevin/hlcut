# hlcut — overview

hlcut is a high-performance Go CLI that takes a **local MP4** (podcasts/tutorials) and produces **highlight clips**, with optional **TikTok-style ASS karaoke subtitles** (`--burn-subtitles`, word-by-word highlight), using:

- **ffmpeg/ffprobe** for media I/O and rendering
- **whisper.cpp** for on-device ASR + timestamps
- **OpenRouter** (LLM) to rank/refine candidate highlight windows

## Why
Most long videos are 90% filler. hlcut’s goal is to turn long-form content into short clips that are ready to post, with optional burned-in karaoke subtitles.

## MVP scope (current)
- Input: `hlcut <input.mp4>` (positional argument)
- Output: `out/<run-id>/` directory with:
  - `manifest.json`
  - `clips/*.mp4` (h264+aac)
  - optional `subtitles/*.ass` (karaoke `\k` tags) when `--burn-subtitles` is enabled
- Default clip duration range: **20..60s** (hidden flags `--min` and `--max`)
- `--clips` is the maximum number of returned highlights (not exact)
- `--burn-subtitles` controls subtitle sidecars + burned-in render (default: `false`)
- Selected clips are distinct and non-overlapping
- If no valid highlights exist, run still succeeds with an empty `manifest.clips`
- LLM is required (OpenRouter API key via `.env`)

## Non-goals (for now)
- URLs as input
- GPU acceleration
- Fancy UI, auto-upload, multi-language
