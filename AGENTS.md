# Repository Guidelines

## Project Structure & Module Organization
- `cmd/hlcut/main.go` is the CLI entrypoint.
- `internal/cli` handles Cobra commands and runtime config.
- `internal/pipeline` orchestrates end-to-end execution.
- `internal/usecase` and `internal/domain/*` contain core business logic.
- `internal/ports` defines interfaces; `internal/ports/adapters/*` contains concrete integrations (`ffmpeg`, `whispercpp`, `openrouter`).
- `internal/itest` contains integration tests guarded by build tags.
- `docs/` stores product and architecture notes. Generated runtime artifacts go to `.cache/` and `out/` (both gitignored).

## Build, Test, and Development Commands
- `make env_up`: build Docker env image.
- `make setup`: prepare whisper/model cache in Docker.
- `make run_local INPUT=... [ARGS="..."]`: run on host.
- `make run_docker INPUT=... [ARGS="..."]`: run in Docker (`.env` + `OPENROUTER_API_KEY` required).
- `make test`: unit tests in Docker.
- `make itest`: integration tests in Docker (`.env` + `OPENROUTER_API_KEY` required).
- `make fmt`: `gofmt -w .` in Docker.
- `make lint`: `golangci-lint` on host.
- `make lint_fix`: install lint tools + run `goimports`, `go fmt`, `golangci-lint`.
- `make hooks_install`: install pre-commit hook.

## Coding Style & Naming Conventions
- Go version is `1.23`; always run `gofmt` before pushing.
- Follow standard Go naming: package names lowercase, exported identifiers `PascalCase`, internals `camelCase`.
- Keep domain logic in `internal/domain` and dependency boundaries via `internal/ports`.
- Prefer small, focused functions and explicit config defaults close to CLI/pipeline construction.

## Testing Guidelines
- Place unit tests next to code in `*_test.go` files; keep them fast and deterministic.
- Use table-driven tests where practical (especially for domain scoring/subtitle logic).
- Keep external dependencies (network, ffmpeg, whisper, OpenRouter) in integration tests under `internal/itest`.
- Minimum check before PR: `make test`. If pipeline/adapters changed, also run `make itest`.

## Commit & Pull Request Guidelines
- Match existing commit style: `scope: concise description` (examples: `docs: ...`, `itest: ...`, `config: ...`).
- Keep commits single-purpose and readable.
- PRs should include: change summary, rationale, test commands run, and output impact (for example `out/manifest.json` behavior changes).

## Security & Configuration Tips
- Never commit `.env`, API keys, or cache/model artifacts.
- Copy `.env.example` to `.env`; set `OPENROUTER_API_KEY` at minimum.
- Use `docker/env/Dockerfile` and `scripts/setup.sh` as the reproducible environment baseline.
