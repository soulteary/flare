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
HTTP_STACK="${HTTP_STACK:-fiber}"

GO_RUN=(go run)
case "${HTTP_STACK}" in
  fiber) ;;
  *)
    echo "HTTP_STACK must be fiber (Echo is available in the historical comparison snapshot) (got: ${HTTP_STACK})" >&2
    exit 2
    ;;
esac

mkdir -p "${OUTPUT_DIR}"

REPORT_FILE="${OUTPUT_DIR}/baseline-report.md"

cat > "${REPORT_FILE}" <<EOF
# Flare Baseline Metrics

- generated_at: $(date -u +"%Y-%m-%dT%H:%M:%SZ")
- requests: ${REQUESTS}
- concurrency: ${CONCURRENCY}
- warmup: ${WARMUP}
- http_stack: ${HTTP_STACK}
- allocation_scope: mixed client and server process
- client_idle_connections_per_host: 2 (original net/http default)

Allocation and profile data include both the load generator and server.
Use scripts/analyze-redirect.py for current isolated redirect resources.
Use scripts/compare-http-stacks.py only to replay the frozen Echo/Fiber experiment.

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
  "${GO_RUN[@]}" ./tools/baseline \
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
