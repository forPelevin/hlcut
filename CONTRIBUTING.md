# Contributing To hlcut

Thanks for contributing.

## Development Setup

```bash
make setup
```

This builds the Docker dev image and prepares whisper artifacts in `.cache/`.

## Coding Rules

- Keep changes focused and single-purpose.
- Follow existing package boundaries (`internal/domain`, `internal/ports`, `internal/usecase`, `internal/pipeline`).
- Run `gofmt` (`make fmt`) before submitting.
- Prefer small, explicit functions over clever abstractions.

## Test Requirements

Minimum before PR:

```bash
make test
```

If you touched pipeline/adapters/external integrations, also run:

```bash
make itest
```

Integration tests require `.env` with `OPENROUTER_API_KEY`.

## Commit Style

Use:

```text
scope: concise description
```

Examples:

- `docs: improve quickstart`
- `itest: run cli e2e on sample fixture`
- `openrouter: harden response parsing`

## Pull Requests

- Explain what changed and why.
- Include commands you ran and results.
- Call out behavior/output changes (`out/manifest.json`, clip boundaries, subtitle format, etc.).
- Keep unrelated refactors out of the same PR.
