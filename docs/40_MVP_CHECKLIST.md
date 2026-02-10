# MVP checklist

## Works when
- `make env_up && make setup` succeeds
- `hlcut <input.mp4>` produces:
  - `out/manifest.json`
  - at least one `out/clips/*.mp4`
  - matching `out/subs/*.ass`
- Clips have **burned-in** subtitles
- Subtitles contain `{\k...}` tags (word-highlight)
- Clip duration is **<= 60s** by default

## Known rough edges
- Integration tests can be flaky due to external LLM behavior / network latency.
- Candidate selection quality is still heuristic-heavy.
