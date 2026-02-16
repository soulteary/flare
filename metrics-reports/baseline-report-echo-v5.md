# Flare Baseline Metrics (Echo V5)

- generated_at: 2026-02-16T08:05:30Z
- requests: 2000
- concurrency: 8
- warmup: 100

| scenario | endpoint | QPS | P50 | P95 | P99 | allocs/op | B/op |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: |
| home | GET / | 9225.43 | 851µs | 1.372ms | 2.001ms | 2602.23 | 228834.02 |
| home-search | POST / | 9133.92 | 866µs | 1.451ms | 1.911ms | 2631.38 | 233986.88 |
| bookmarks | GET /bookmarks | 11000.46 | 684µs | 1.313ms | 1.859ms | 2094.21 | 186453.62 |
| applications | GET /applications | 17278.75 | 412µs | 893µs | 1.179ms | 859.72 | 97003.07 |
| redir-url | GET /redir/url?go=aHR0cHM6Ly9saW5rLmV4YW1wbGUuY29t | 30330.47 | 204µs | 596µs | 1.048ms | 450.91 | 31038.76 |
