#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUTPUT_DIR="${1:-${ROOT_DIR}/.benchmark/baseline-$(date +%Y%m%d-%H%M%S)}"
if [[ "${OUTPUT_DIR}" != /* ]]; then
  OUTPUT_DIR="${ROOT_DIR}/${OUTPUT_DIR}"
fi

REQUESTS="${REQUESTS:-2000}"
CONCURRENCY="${CONCURRENCY:-8}"
WARMUP="${WARMUP:-100}"

mkdir -p "${OUTPUT_DIR}"

REPORT_FILE="${OUTPUT_DIR}/baseline-report.md"

cat > "${REPORT_FILE}" <<EOF
# Flare Baseline Metrics

- generated_at: $(date -u +"%Y-%m-%dT%H:%M:%SZ")
- requests: ${REQUESTS}
- concurrency: ${CONCURRENCY}
- warmup: ${WARMUP}

| scenario | endpoint | QPS | P50 | P95 | P99 | allocs/op | B/op |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: |
EOF

SCENARIOS=(
  "home"
  "home-search"
  "bookmarks"
  "applications"
  "redir-url"
)

for scenario in "${SCENARIOS[@]}"; do
  echo "running scenario: ${scenario}"
  go run ./tools/baseline \
    -scenario "${scenario}" \
    -requests "${REQUESTS}" \
    -concurrency "${CONCURRENCY}" \
    -warmup "${WARMUP}" \
    -cpuprofile "${OUTPUT_DIR}/${scenario}.cpu.pprof" \
    -memprofile "${OUTPUT_DIR}/${scenario}.mem.pprof" \
    -markdown | awk '/^\|/' >> "${REPORT_FILE}"
done

cat <<EOF
Baseline metrics finished.

Output directory: ${OUTPUT_DIR}
Markdown report: ${REPORT_FILE}

Inspect CPU profile:
  go tool pprof -top ${OUTPUT_DIR}/home.cpu.pprof

Inspect heap profile:
  go tool pprof -top ${OUTPUT_DIR}/home.mem.pprof
EOF
