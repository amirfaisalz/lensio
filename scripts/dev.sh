#!/usr/bin/env bash
set -eo pipefail

# Determine repository root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${ROOT_DIR}"

# Load environment variables from .env if present
if [ -f ".env" ]; then
    export $(grep -v '^#' .env | xargs)
fi

API_PORT="${PORT:-8080}"
DASHBOARD_PORT="${DASHBOARD_PORT:-3000}"
DATABASE_URL="${DATABASE_URL:-postgres://lensio:lensio_dev_password@localhost:5432/lensio?sslmode=disable}"
KEYCLOAK_PORT="${KEYCLOAK_PORT:-8082}"
KEYCLOAK_JWKS_URL="${KEYCLOAK_JWKS_URL:-http://localhost:${KEYCLOAK_PORT}/realms/lensio/protocol/openid-connect/certs}"

# Ensure PostgreSQL and Keycloak containers are running if docker is available
if command -v docker >/dev/null 2>&1; then
    if ! docker compose ps --status running --format '{{.Service}}' 2>/dev/null | grep -q "postgres"; then
        echo "⏳ Starting PostgreSQL container with Docker Compose..."
        docker compose up -d postgres >/dev/null 2>&1 || true
    fi
    if ! docker compose ps --status running --format '{{.Service}}' 2>/dev/null | grep -q "keycloak"; then
        echo "⏳ Starting Keycloak container with Docker Compose..."
        docker compose up -d keycloak >/dev/null 2>&1 || true
    fi
fi

# Track background process IDs
API_PID=""
DASH_PID=""

cleanup() {
    echo ""
    echo "🛑 Shutting down Lensio development services..."
    if [ -n "$API_PID" ] && kill -0 "$API_PID" 2>/dev/null; then
        kill "$API_PID" 2>/dev/null || true
    fi
    if [ -n "$DASH_PID" ] && kill -0 "$DASH_PID" 2>/dev/null; then
        kill "$DASH_PID" 2>/dev/null || true
    fi
    wait "$API_PID" "$DASH_PID" 2>/dev/null || true
    echo "👋 All services stopped cleanly."
    exit 0
}

trap cleanup SIGINT SIGTERM EXIT

# Start Go API server
PORT="${API_PORT}" DATABASE_URL="${DATABASE_URL}" KEYCLOAK_JWKS_URL="${KEYCLOAK_JWKS_URL}" go run ./apps/api/cmd/server &
API_PID=$!

# Start React Dashboard with bun
(cd apps/dashboard && bun run dev --port "${DASHBOARD_PORT}") &
DASH_PID=$!

# Allow a moment for initial server binding
sleep 1

# Display prominent terminal banner with server and dashboard ports
echo ""
echo "========================================================"
echo "  🚀 Lensio Local Development Environment Running"
echo "========================================================"
echo "  🌐 React Dashboard:  http://localhost:${DASHBOARD_PORT}"
echo "  ⚡ Go API Server:    http://localhost:${API_PORT}"
echo "  🔐 Keycloak OIDC:    http://localhost:${KEYCLOAK_PORT}"
echo "  🩺 Liveness Probe:   http://localhost:${API_PORT}/health"
echo "  📊 Readiness Probe:  http://localhost:${API_PORT}/ready"
echo "========================================================"
echo "  Streaming logs below. Press [Ctrl+C] to stop all."
echo "========================================================"
echo ""

# Wait on background processes
wait "$API_PID" "$DASH_PID"
