# hlcut

![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go)
![Status](https://img.shields.io/badge/status-MVP-orange)
![Input](https://img.shields.io/badge/input-local%20mp4-blue)
![License](https://img.shields.io/badge/license-Apache--2.0-blue)

`hlcut` is a local-first CLI that turns long podcast/tutorial videos into short highlight clips, with optional burned-in karaoke subtitles (`--burn-subtitles`).

No URL downloading. Bring your own local `.mp4`.

## Table Of Contents

- [Why](#why)
- [Run Modes & Dependencies](#run-modes--dependencies)
- [Quick Start](#quick-start)
- [Run CLI](#run-cli)
- [Output](#output)
- [Configuration](#configuration)
- [How It Works](#how-it-works)
- [Documentation](#documentation)
- [Roadmap](#roadmap)
- [Development](#development)
- [Testing](#testing)
- [Troubleshooting](#troubleshooting)
- [Support](#support)
- [Contributing](#contributing)
- [Security](#security)
- [License](#license)

## Why

- Long-form videos are mostly filler; good moments are sparse.
- Manual clipping + subtitle timing is slow.
- `hlcut` automates the pipeline end-to-end with local media processing and LLM-assisted clip ranking.

## Run Modes & Dependencies

`hlcut` has two valid run modes, and they do not share the same runtime dependencies.

| Run mode | Where CLI executes | Required dependencies |
|---|---|---|
| Docker | Inside `hlcut-env:local` container | Docker, `.env` with `OPENROUTER_API_KEY` |
| Localhost | On your host OS (`go run` or built binary) | Go `1.26`, `ffmpeg` + `ffprobe`, C/C++ toolchain + `cmake`, `git`, `curl`, `.env` with `OPENROUTER_API_KEY` |

Important: `make setup` runs inside Docker and produces Linux whisper artifacts in `.cache/`. Use those artifacts for Docker runs. For localhost runs, build whisper artifacts on localhost.

## Quick Start

### Docker (recommended)

1. Create env file:

```bash
cp .env.example .env
```

2. Set `OPENROUTER_API_KEY` in `.env`.

```dotenv
OPENROUTER_API_KEY=your_key_here
```

3. Build runtime image + whisper artifacts:

```bash
make setup
```

4. Run CLI in Docker on sample fixture:

```bash
docker run --rm \
  --env-file .env \
  -v "$PWD":/work \
  -w /work \
  hlcut-env:local \
  bash -c "go run ./cmd/hlcut ./internal/itest/testdata/podcast_short.mp4 --out ./out --clips 2 --burn-subtitles"
```

Expected artifacts:

```text
out/<run-id>/manifest.json
out/<run-id>/clips/*.mp4
# only with --burn-subtitles:
out/<run-id>/subtitles/*.ass
```

### Localhost

1. Install host dependencies:
   - Go `1.26`
   - `ffmpeg` and `ffprobe`
   - `cmake`, C/C++ compiler toolchain, `git`, `curl`

2. Prepare whisper artifacts on localhost:

```bash
bash ./scripts/setup.sh
```

If CMake fails on OpenMP support, rerun configure with `-DGGML_OPENMP=OFF`.

3. Run CLI locally:

```bash
go run ./cmd/hlcut /absolute/path/to/input.mp4 --out ./out --clips 6
```

## Run CLI

```bash
hlcut <input> [flags]
```

Flags:

- `--out` output directory (default: `out`)
- `--clips` max number of clips to return (auto-adjusted from duration when flag is omitted; minimum default still applies)
- `--burn-subtitles` burn karaoke subtitles into clips and write `<run-dir>/subtitles/*.ass` (default: `false`)

Examples:

Docker (input inside repo):

```bash
docker run --rm \
  --env-file .env \
  -v "$PWD":/work \
  -w /work \
  hlcut-env:local \
  bash -c "go run ./cmd/hlcut ./internal/itest/testdata/podcast_short.mp4 --out ./out --clips 2"
```

Docker (input outside repo):

```bash
docker run --rm \
  --env-file .env \
  -v "$PWD":/work \
  -v "/absolute/path/to/media":/media:ro \
  -w /work \
  hlcut-env:local \
  bash -c "go run ./cmd/hlcut /media/input.mp4 --out ./out --clips 6"
```

Localhost:

```bash
go run ./cmd/hlcut /absolute/path/to/input.mp4 --out ./out --clips 6
```

Localhost (with burned subtitles):

```bash
go run ./cmd/hlcut /absolute/path/to/input.mp4 --out ./out --clips 6 --burn-subtitles
```

```bash
go build -o ./bin/hlcut ./cmd/hlcut
./bin/hlcut /absolute/path/to/input.mp4 --out ./out --clips 6
```

## Output

```text
out/
  <unix_ms>-<video-name>-<id>/
    manifest.json
    clips/
      001.mp4
      002.mp4
      ...
    subtitles/         # only with --burn-subtitles
      001.ass
      002.ass
      ...
```

`manifest.json` contains clip timing and metadata (title/caption/tags/file paths). Each run gets a fresh subdirectory under `--out`.

Behavior guarantees:

- `--clips` is an upper bound, not an exact target.
- Clips are constrained by internal duration policy (currently `20..180s`) and non-overlapping.
- If no valid highlights exist, run completes successfully and writes an empty `clips` array in `manifest.json`.
- No cleanup of previous runs: every run writes to a new run directory inside `--out`.

## Configuration

Environment variables:

- `OPENROUTER_API_KEY` (required)
- `OPENROUTER_MODEL` (optional, default: `z-ai/glm-4.5-air:free`)
- `OPENROUTER_BASE_URL` (optional, default: `https://openrouter.ai`)
- `OPENROUTER_ALLOWED_HOSTS` (optional, default: `openrouter.ai,api.openrouter.ai`)

`.env` is auto-loaded by the CLI.

## How It Works

1. Extract audio from MP4 using `ffmpeg`.
2. Transcribe using `whisper.cpp` with word timing.
3. Build candidate highlight windows from transcript segments/words.
4. Ask OpenRouter model to refine/select distinct highlight clips (bounded by `--clips`, constrained by internal duration policy).
5. Optionally render ASS karaoke subtitles (`--burn-subtitles`).
6. Render final clips via `ffmpeg` (subtitle burn-in only when `--burn-subtitles` is set).
7. Write `manifest.json`.

## Documentation

- Overview: `docs/00_OVERVIEW.md`
- Features: `docs/10_FEATURES.md`
- Architecture: `docs/20_ARCHITECTURE.md`
- Technical details: `docs/30_TECHNICAL_DETAILS.md`
- MVP checklist: `docs/40_MVP_CHECKLIST.md`

## Roadmap

Near-term targets:

- Better semantic topic diversity in clip selection
- Better sentence-aware clip boundaries
- Multiple subtitle style presets
- URL input support (download + caching)
- Config file support while keeping CLI stable

## Development

Development commands:

```bash
make env_up     # build Docker dev image
make setup      # build whisper.cpp + model in Docker into .cache/
make run_docker INPUT=./internal/itest/testdata/podcast_short.mp4 ARGS="--out ./out --clips 2"
make run_local  INPUT=./internal/itest/testdata/podcast_short.mp4 ARGS="--out ./out --clips 2"
make test       # unit tests in Docker
make itest      # integration tests in Docker (real ffmpeg + whisper.cpp + OpenRouter)
make fmt        # gofmt in container
```

Optional lint tooling:

```bash
make lint_fix   # install goimports + golangci-lint, then format + lint
make lint
```

## Testing

- Unit tests are fast/deterministic.
- Integration tests run the real CLI end-to-end.
- Integration fixture: `internal/itest/testdata/podcast_short.mp4`.
- Integration tests require:
  - valid `.env` with `OPENROUTER_API_KEY`
  - whisper artifacts from `make setup`

## Troubleshooting

- `OPENROUTER_API_KEY is required`:
  - Add it to `.env`.
- `missing whisper model/binary`:
  - Docker mode: run `make setup`.
  - Localhost mode: build whisper/model in `.cache/` with the localhost steps from Quick Start.
- Docker daemon errors:
  - Start Docker and retry.
- Slow/flaky integration runs:
  - `make itest` depends on network + external OpenRouter response time.

## Support

- Bug reports and feature requests:
  - [GitHub Issues](https://github.com/forPelevin/hlcut/issues)
- Design and architecture docs:
  - `docs/`

## Contributing

PRs are welcome.

See `CONTRIBUTING.md` for full workflow and PR expectations.

## Security

See `SECURITY.md` for vulnerability reporting guidance.

## License

Licensed under the Apache License, Version 2.0. See `LICENSE`.
