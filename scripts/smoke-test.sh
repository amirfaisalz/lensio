#!/usr/bin/env bash
# ==============================================================================
# NusaID Production Smoke Test Suite
# ==============================================================================
# Verifies system health, readiness, and core KTP OCR API functionality.
# Used by CI/CD pipelines (post-deployment) and automated failure drills.
# ==============================================================================

set -euo pipefail

# Default configuration
API_URL="${1:-${API_URL:-http://localhost:8080}}"
DASHBOARD_URL="${2:-${DASHBOARD_URL:-http://localhost:3000}}"
API_KEY="${API_KEY:-}"
MAX_RETRIES="${MAX_RETRIES:-12}"
RETRY_DELAY_SEC="${RETRY_DELAY_SEC:-5}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_pass() {
    echo -e "${GREEN}[PASS]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_fail() {
    echo -e "${RED}[FAIL]${NC} $1" >&2
}

echo "========================================================"
echo " 🩺 NusaID Automated Smoke Test Suite"
echo "========================================================"
echo "  Target API URL:       ${API_URL}"
echo "  Target Dashboard URL: ${DASHBOARD_URL}"
echo "  Max Retries:          ${MAX_RETRIES} (${RETRY_DELAY_SEC}s delay)"
echo "========================================================"

# ------------------------------------------------------------------------------
# Test 1: Liveness Probe (/health)
# ------------------------------------------------------------------------------
log_info "Verifying API process liveness (/health)..."
HEALTH_OK=false
for i in $(seq 1 "${MAX_RETRIES}"); do
    HEALTH_RESP=$(curl -s -w "\n%{http_code}" --connect-timeout 3 "${API_URL}/health" || true)
    HTTP_CODE=$(echo "${HEALTH_RESP}" | tail -n1)
    BODY=$(echo "${HEALTH_RESP}" | sed '$d')

    if [ "${HTTP_CODE}" = "200" ] && echo "${BODY}" | grep -q '"status":"ok"'; then
        HEALTH_OK=true
        break
    fi
    log_warn "Attempt ${i}/${MAX_RETRIES} failed (HTTP ${HTTP_CODE}). Retrying in ${RETRY_DELAY_SEC}s..."
    sleep "${RETRY_DELAY_SEC}"
done

if [ "${HEALTH_OK}" = false ]; then
    log_fail "Liveness probe failed at ${API_URL}/health. Response: ${HEALTH_RESP}"
    exit 1
fi
log_pass "Liveness probe returned HTTP 200 OK."

# ------------------------------------------------------------------------------
# Test 2: Readiness Probe (/ready)
# ------------------------------------------------------------------------------
log_info "Verifying API & database readiness (/ready)..."
READY_OK=false
for i in $(seq 1 "${MAX_RETRIES}"); do
    READY_RESP=$(curl -s -w "\n%{http_code}" --connect-timeout 3 "${API_URL}/ready" || true)
    HTTP_CODE=$(echo "${READY_RESP}" | tail -n1)
    BODY=$(echo "${READY_RESP}" | sed '$d')

    if [ "${HTTP_CODE}" = "200" ] && echo "${BODY}" | grep -q '"database":"connected"'; then
        READY_OK=true
        break
    fi
    log_warn "Attempt ${i}/${MAX_RETRIES} failed (HTTP ${HTTP_CODE}). Retrying in ${RETRY_DELAY_SEC}s..."
    sleep "${RETRY_DELAY_SEC}"
done

if [ "${READY_OK}" = false ]; then
    log_fail "Readiness probe failed at ${API_URL}/ready. Response: ${READY_RESP}"
    exit 1
fi
log_pass "Readiness probe returned HTTP 200 (database connected)."

# ------------------------------------------------------------------------------
# Test 3: OCR KTP Endpoint Functional Smoke Test
# ------------------------------------------------------------------------------
log_info "Verifying OCR extraction pipeline (/api/v1/ocr/ktp)..."

# Generate 1x1 synthetic PNG in temporary file
TEMP_IMG=$(mktemp /tmp/nusaid_synthetic_XXXXXX.png)
echo "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==" | base64 -d > "${TEMP_IMG}"

# If API_KEY not provided, test that endpoint correctly enforces authentication (401 Unauthorized)
if [ -z "${API_KEY}" ]; then
    log_info "No API_KEY provided in environment; verifying 401 Unauthorized guard on /api/v1/ocr/ktp..."
    OCR_RESP=$(curl -s -w "\n%{http_code}" -X POST \
        -F "document=@${TEMP_IMG};type=image/png" \
        "${API_URL}/api/v1/ocr/ktp" || true)
    HTTP_CODE=$(echo "${OCR_RESP}" | tail -n1)
    BODY=$(echo "${OCR_RESP}" | sed '$d')

    if [ "${HTTP_CODE}" = "401" ] && echo "${BODY}" | grep -q '"code":"invalid_api_key"'; then
        log_pass "OCR endpoint correctly authenticated and rejected unauthenticated request (HTTP 401)."
    else
        log_fail "OCR security guard verification failed. Expected 401, got HTTP ${HTTP_CODE}. Body: ${BODY}"
        rm -f "${TEMP_IMG}"
        exit 1
    fi
else
    # Authenticated OCR test
    log_info "Executing authenticated OCR smoke test with synthetic document..."
    OCR_RESP=$(curl -s -w "\n%{http_code}" -X POST \
        -H "Authorization: Bearer ${API_KEY}" \
        -F "document=@${TEMP_IMG};type=image/png" \
        "${API_URL}/api/v1/ocr/ktp" || true)
    HTTP_CODE=$(echo "${OCR_RESP}" | tail -n1)
    BODY=$(echo "${OCR_RESP}" | sed '$d')

    # Status 200, 422 (unsupported test doc), or 200 ok is expected with synthetic image
    if [ "${HTTP_CODE}" = "200" ] || [ "${HTTP_CODE}" = "422" ]; then
        log_pass "OCR endpoint processed request successfully (HTTP ${HTTP_CODE})."
    else
        log_fail "OCR functional smoke test failed. Got HTTP ${HTTP_CODE}. Body: ${BODY}"
        rm -f "${TEMP_IMG}"
        exit 1
    fi
fi
rm -f "${TEMP_IMG}"

# ------------------------------------------------------------------------------
# Test 4: Dashboard Health / Accessibility (Optional if dashboard URL provided)
# ------------------------------------------------------------------------------
if [ -n "${DASHBOARD_URL}" ]; then
    log_info "Verifying Dashboard accessibility (${DASHBOARD_URL})..."
    DASH_RESP=$(curl -s -w "%{http_code}" -o /dev/null --connect-timeout 3 "${DASHBOARD_URL}" || true)
    if [ "${DASH_RESP}" = "200" ] || [ "${DASH_RESP}" = "301" ] || [ "${DASH_RESP}" = "302" ]; then
        log_pass "Dashboard endpoint reachable (HTTP ${DASH_RESP})."
    else
        log_warn "Dashboard returned HTTP ${DASH_RESP} (non-blocking for API smoke test)."
    fi
fi

echo "========================================================"
echo -e "${GREEN}🎉 All Smoke Tests Passed Cleanly! System is Healthy.${NC}"
echo "========================================================"
exit 0
