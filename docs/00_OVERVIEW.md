# hlcut — overview

hlcut is a high-performance Go CLI that takes a **local MP4** (podcasts/tutorials) and produces **highlight clips** with **TikTok-style ASS karaoke subtitles** (word-by-word highlight), using:

- **ffmpeg/ffprobe** for media I/O and rendering
- **whisper.cpp** for on-device ASR + timestamps
- **OpenRouter** (LLM) to rank/refine candidate highlight windows

## Why
Most long videos are 90% filler. hlcut’s goal is to turn long-form content into short, subtitle-burned clips that are ready to post.

## MVP scope (current)
- Input: `hlcut <input.mp4>` (positional argument)
- Output: `out/` directory with:
  - `manifest.json`
  - `clips/*.mp4` (h264+aac)
  - `subs/*.ass` (karaoke `\k` tags)
- Default max clip length: **60s** (hidden flag `--max`)
- LLM is required (OpenRouter API key via `.env`)

## Non-goals (for now)
- URLs as input
- GPU acceleration
- Fancy UI, auto-upload, multi-language
