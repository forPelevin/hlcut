# MVP checklist

## Works when
- `make env_up && make setup` succeeds
- `hlcut <input.mp4>` produces:
  - `out/<run-id>/manifest.json`
  - zero or more `out/<run-id>/clips/*.mp4`
- with `--burn-subtitles`, matching `out/<run-id>/subtitles/*.ass` for each emitted clip
- with `--burn-subtitles`, clips have **burned-in** subtitles and ASS files contain `{\k...}` tags (word-highlight)
- Clip duration is in **20..60s** by default
- Emitted clips are non-overlapping

## Known rough edges
- Integration tests can be flaky due to external LLM behavior / network latency.
- Candidate selection quality is still heuristic-heavy.
