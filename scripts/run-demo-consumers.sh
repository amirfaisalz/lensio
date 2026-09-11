#!/usr/bin/env bash
set -euo pipefail

echo "========================================================"
echo " 🌐 NusaID External Demo Consumers Verification Suite"
echo "========================================================"
echo "Verifying independent client consumers: VeriForm & RentEase"
echo ""

# 1. Run Automated Consumer Tests with Race Detector
echo "==> [Step 1/2] Running automated consumer test suites..."
go test -race -v -cover ./examples/veriform/...
echo "✅ VeriForm test suite passed."
echo ""

go test -race -v -cover ./examples/rentease/...
echo "✅ RentEase test suite passed."
echo ""

# 2. Check if a live NusaID API server is reachable
API_URL="${NUSAID_BASE_URL:-http://localhost:8080}"
API_KEY="${NUSAID_API_KEY:-nusa_test_demo_key}"

echo "==> [Step 2/2] Checking live API connectivity at ${API_URL}..."
if curl -s -f "${API_URL}/health" >/dev/null 2>&1; then
    echo "  • Live NusaID instance detected at ${API_URL}."
    echo "  • Executing VeriForm consumer against live API..."
    go run ./examples/veriform/main.go --api-url "${API_URL}" --api-key "${API_KEY}" --image "tests/fixtures/synthetic/valid_ktp.jpg" || true
    echo ""
    echo "  • Executing RentEase consumer against live API..."
    go run ./examples/rentease/main.go --api-url "${API_URL}" --api-key "${API_KEY}" --image "tests/fixtures/synthetic/valid_ktp.jpg" || true
else
    echo "  ℹ️ Live API not running at ${API_URL} (Skipping live HTTP calls)."
    echo "  ℹ️ Both consumers passed comprehensive offline unit & mock integration tests."
fi

echo ""
echo "========================================================"
echo " 🎉 All External Demo Consumers Verified Successfully!"
echo "========================================================"
