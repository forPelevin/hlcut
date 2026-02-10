# hlcut

A high-performance CLI that takes a **local** MP4 (podcasts/tutorials) and outputs:
- short highlight clips (MP4)
- TikTok-style subtitles (ASS karaoke with word highlight, burned-in by default)

> No YouTube downloading. You provide the local file.

## Quickstart

1) Create `.env` from `.env.example` and set `OPENROUTER_API_KEY`.

2) Prepare dependencies (containerized):

```bash
make setup
```

3) Run unit tests:

```bash
make test
```

4) Run integration tests (real ffmpeg + whisper.cpp + OpenRouter):

```bash
make itest
```

## Usage

```bash
hlcut <input.mp4>
```

Outputs go to `out/` by default.

## Notes

- The default clip max length is 60 seconds (internal/hidden for now).
- All secrets and models live in `.env` and `.cache/` and are ignored by git.

## License

TBD
