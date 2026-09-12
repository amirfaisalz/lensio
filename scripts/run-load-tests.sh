#!/usr/bin/env bash
# ==============================================================================
# Lensio Load Testing Runner
# ==============================================================================
# Executes k6 baseline (100 VUs) and stress (ramp to 1,000 RPS) load tests.
# Supports native k6 or dockerized grafana/k6 automatically.
# ==============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${ROOT_DIR}"

API_URL="${BASE_URL:-http://localhost:8080}"
MODE="${1:-all}" # baseline, stress, or all

BLUE='\033[0;34m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_pass() { echo -e "${GREEN}[PASS]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_fail() { echo -e "${RED}[FAIL]${NC} $1" >&2; }

echo "========================================================"
echo " ⚡ Lensio k6 Load Testing & RED Metrics Runner"
echo "========================================================"
echo "  Target API URL: ${API_URL}"
echo "  Test Mode:      ${MODE}"
echo "========================================================"

# 1. Verify API server is alive
log_info "Verifying target API is reachable at ${API_URL}/health..."
if ! curl -s -f -m 3 "${API_URL}/health" >/dev/null 2>&1; then
    log_fail "Target API is not reachable at ${API_URL}/health."
    log_info "Please ensure the Lensio API server is running (e.g. via ./scripts/dev.sh or go run ./apps/api/cmd/server)."
    exit 1
fi
log_pass "Target API is healthy and reachable."

# 2. Ensure an API key exists or create one for the test run
if [ -z "${API_KEY:-}" ]; then
    log_info "Ensuring active API key for load testing..."
    KEY_RESP=$(curl -s -X POST "${API_URL}/api/v1/auth/api-keys" \
        -H "Content-Type: application/json" \
        -d '{"name":"k6-automated-runner-key","scopes":["ocr:write","ocr:read","usage:read"]}' || true)
    API_KEY=$(echo "${KEY_RESP}" | grep -o '"key":"[^"]*' | cut -d'"' -f4 || true)
    
    if [ -n "${API_KEY}" ]; then
        log_pass "Provisioned dedicated load test API key."
    else
        log_warn "Could not auto-generate key (may be rate limited). k6 setup() will attempt fallback."
    fi
fi

# 3. Determine k6 runner (native or Docker)
RUNNER="native"
if command -v k6 >/dev/null 2>&1; then
    log_info "Using native k6: $(k6 version)"
    K6_CMD=(k6 run)
else
    if command -v docker >/dev/null 2>&1; then
        log_info "Native k6 not found. Using dockerized grafana/k6..."
        K6_CMD=(docker run --rm -i --network=host -v "${ROOT_DIR}:/lensio" -w /lensio/tests/load grafana/k6 run)
        RUNNER="docker"
    else
        log_fail "Neither native k6 nor Docker was found. Please install k6 or Docker."
        exit 1
    fi
fi

run_suite() {
    local script_name="$1"
    local script_path
    if [ "${RUNNER}" = "docker" ]; then
        script_path="${script_name}"
    else
        script_path="tests/load/${script_name}"
    fi

    log_info "Executing ${script_name}..."
    local env_args=()
    if [ -n "${API_KEY:-}" ]; then
        env_args+=(-e "API_KEY=${API_KEY}")
    fi
    "${K6_CMD[@]}" "${env_args[@]}" "${@:2}" "${script_path}"
}

case "${MODE}" in
    baseline)
        log_info "Starting Baseline Suite (100 VUs)..."
        run_suite "k6-baseline.js" -e BASE_URL="${API_URL}" -e VUS="${VUS:-100}" -e DURATION="${DURATION:-2m}"
        log_pass "Baseline Suite completed."
        ;;
    stress)
        log_info "Starting Stress Suite (Ramp-up to 1,000 RPS)..."
        run_suite "k6-stress.js" -e BASE_URL="${API_URL}" -e STAGE_SECS="${STAGE_SECS:-30}"
        log_pass "Stress Suite completed."
        ;;
    all)
        log_info "Starting Full Load Testing Suite..."
        log_info "--- Phase 1: Baseline Test (100 VUs) ---"
        run_suite "k6-baseline.js" -e BASE_URL="${API_URL}" -e VUS="${VUS:-100}" -e DURATION="${DURATION:-2m}"
        echo ""
        log_info "--- Phase 2: Stress Test (Ramp-up to 1,000 RPS) ---"
        run_suite "k6-stress.js" -e BASE_URL="${API_URL}" -e STAGE_SECS="${STAGE_SECS:-30}"
        log_pass "All load testing suites completed successfully."
        ;;
    *)
        log_fail "Unknown mode '${MODE}'. Choose 'baseline', 'stress', or 'all'."
        exit 1
        ;;
esac
