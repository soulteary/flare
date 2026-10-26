# Additional code conventions

Flare targets lightweight self-hosted deployments. Keep implementation and
operational requirements simple, and measure changes on representative data.

- Prefer straightforward functions and small, cohesive packages.
- Choose pointer or value receivers according to ownership and mutation needs;
  neither choice alone establishes concurrency safety.
- Avoid unnecessary allocations, but preserve request-data ownership and
  concurrency guarantees. Use profiles before adding caches or pooling.
- Define data invariants and cover meaningful behavior changes with regression
  tests. Do not change unrelated code solely to enforce a stylistic preference.
- Report performance with the source commit, workload, environment and measurement
  scope. Allocated bytes, retained heap and RSS are different metrics.

See [contributing](CONTRIBUTING.md) for required checks and
[measurement methods](docs/baseline-metrics.md) for performance experiments.
