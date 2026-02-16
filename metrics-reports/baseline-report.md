# Flare Baseline Metrics

- generated_at: 2026-02-16T06:22:08Z
- requests: 2000
- concurrency: 8
- warmup: 100

| scenario | endpoint | QPS | P50 | P95 | P99 | allocs/op | B/op |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: |
| home | GET / | 6791.32 | 1.103ms | 2.134ms | 2.811ms | 2692.08 | 221251.98 |
| home-search | POST / | 6377.91 | 1.089ms | 2.408ms | 3.986ms | 2731.78 | 229567.46 |
| bookmarks | GET /bookmarks | 10161.65 | 723µs | 1.51ms | 2.094ms | 1996.02 | 154565.82 |
| applications | GET /applications | 13523.05 | 538µs | 1.134ms | 1.49ms | 1044.93 | 86402.31 |
| redir-url | GET /redir/url?go=aHR0cHM6Ly9saW5rLmV4YW1wbGUuY29t | 28429.18 | 223µs | 627µs | 999µs | 464.15 | 31738.56 |
