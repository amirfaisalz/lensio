#!/usr/bin/env bash
# ==============================================================================
# Lensio Playwright End-to-End Test Suite Runner (Phase 9 - PRD Section 28)
# ==============================================================================
# Orchestrates local/CI environment:
#   1. Starts Lensio Go API on port 8080
#   2. Starts React Dashboard on port 3000
#   3. Executes Playwright test suite (6 critical user journeys)
#   4. Gracefully terminates all background services on exit
# ==============================================================================

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

API_PID=""
DASH_PID=""

cleanup() {
    echo ""
    echo "🧹 Tearing down E2E test services..."
    if [ -n "${API_PID}" ]; then
        kill "${API_PID}" 2>/dev/null || true
    fi
    if [ -n "${DASH_PID}" ]; then
        kill "${DASH_PID}" 2>/dev/null || true
    fi
}
trap cleanup EXIT INT TERM

PORT=8080
API_URL="http://localhost:${PORT}"
DASHBOARD_URL="http://localhost:3000"
DB_URL="${DATABASE_URL:-postgres://lensio:lensio_dev_password@localhost:5432/lensio?sslmode=disable}"

echo "======================================================================"
echo " 🎭 Lensio Playwright E2E Suite Runner (PRD Section 28)"
echo "======================================================================"

# 1. Start Go API Server
echo "🚀 Starting Lensio API on ${API_URL}..."
PORT="${PORT}" DATABASE_URL="${DB_URL}" ENV="test" LOG_LEVEL="warn" \
    go run ./apps/api/cmd/server > /tmp/lensio_api_e2e.log 2>&1 &
API_PID=$!

echo "Waiting for API server readiness..."
READY=false
for i in $(seq 1 30); do
    if curl -s "${API_URL}/ready" | grep -q '"database":"connected"'; then
        READY=true
        echo "✅ API server and database ready!"
        if command -v docker >/dev/null 2>&1 && docker ps | grep -q lensio-postgres; then
            docker exec lensio-postgres psql -U lensio -d lensio -c \
                "UPDATE organizations SET plan_id = (SELECT id FROM plans WHERE code = 'pro') WHERE id = '00000000-0000-0000-0000-000000000001';" >/dev/null 2>&1 || true
        elif command -v psql >/dev/null 2>&1; then
            psql "${DB_URL}" -c \
                "UPDATE organizations SET plan_id = (SELECT id FROM plans WHERE code = 'pro') WHERE id = '00000000-0000-0000-0000-000000000001';" >/dev/null 2>&1 || true
        fi
        break
    fi
    sleep 1
done

if [ "${READY}" = false ]; then
    echo "❌ API server failed to become ready. Log output:"
    cat /tmp/lensio_api_e2e.log
    exit 1
fi

# 2. Start Dashboard Vite Dev Server
echo "🚀 Starting React Dashboard on ${DASHBOARD_URL}..."
npm --prefix apps/dashboard run dev -- --port 3000 > /tmp/lensio_dash_e2e.log 2>&1 &
DASH_PID=$!

echo "Waiting for Dashboard server accessibility..."
DASH_READY=false
for i in $(seq 1 30); do
    if curl -s "${DASHBOARD_URL}" > /dev/null 2>&1; then
        DASH_READY=true
        echo "✅ Dashboard frontend server ready!"
        break
    fi
    sleep 1
done

if [ "${DASH_READY}" = false ]; then
    echo "❌ Dashboard failed to start. Log output:"
    cat /tmp/lensio_dash_e2e.log
    exit 1
fi

# 3. Execute Playwright Tests
echo ""
echo "▶️ Executing Playwright E2E Tests..."
export API_URL="${API_URL}"
export DASHBOARD_URL="${DASHBOARD_URL}"

npx --prefix tests/e2e playwright test --config tests/e2e/playwright.config.ts tests/e2e

echo ""
echo "======================================================================"
echo "🎉 All Playwright E2E User Journeys Passed Successfully!"
echo "======================================================================"
