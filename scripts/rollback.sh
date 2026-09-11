#!/usr/bin/env bash
# ==============================================================================
# Lensio Azure Container Apps Rapid Rollback Automation
# ==============================================================================
# Enables instantaneous traffic shifting (<60 seconds SLA) to a previous stable
# container revision on Azure Container Apps without rebuilding images.
# ==============================================================================

set -euo pipefail

ENV="production"
APP="api"
TARGET_REVISION=""
TRAFFIC="100"
RESOURCE_GROUP=""
DRY_RUN=false
VERIFY=true
API_URL=""

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

usage() {
    cat <<EOF
Usage: $(basename "$0") [OPTIONS]

Options:
  -e, --env <staging|production>   Target deployment environment (default: production)
  -a, --app <api|dashboard|all>    Application component to roll back (default: api)
  -r, --target-revision <name>     Target Azure Container App revision name (required)
  -t, --traffic <percentage>       Traffic percentage to shift (default: 100)
  -g, --resource-group <name>      Azure Resource Group (default: rg-lensio-<env>)
  -u, --api-url <url>              API base URL for post-rollback verification
  --dry-run                        Simulate rollback without executing Azure commands
  --no-verify                      Skip post-rollback health checks
  -h, --help                       Show this help message

Examples:
  $(basename "$0") --env production --app api --target-revision ca-api-lensio-prod--20260912-1400
  $(basename "$0") --env staging --target-revision ca-api-lensio-staging--stable --dry-run
EOF
    exit 1
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        -e|--env)
            ENV="$2"; shift 2 ;;
        -a|--app)
            APP="$2"; shift 2 ;;
        -r|--target-revision)
            TARGET_REVISION="$2"; shift 2 ;;
        -t|--traffic)
            TRAFFIC="$2"; shift 2 ;;
        -g|--resource-group)
            RESOURCE_GROUP="$2"; shift 2 ;;
        -u|--api-url)
            API_URL="$2"; shift 2 ;;
        --dry-run)
            DRY_RUN=true; shift ;;
        --no-verify)
            VERIFY=false; shift ;;
        -h|--help)
            usage ;;
        *)
            echo "Unknown argument: $1" >&2
            usage ;;
    esac
done

if [ -z "${RESOURCE_GROUP}" ]; then
    RESOURCE_GROUP="rg-lensio-${ENV}"
fi

if [ -z "${TARGET_REVISION}" ]; then
    echo -e "${RED}[ERROR]${NC} --target-revision is required." >&2
    usage
fi

START_TIME=$(date +%s)

echo "========================================================"
echo " 🚨 Lensio Emergency Revision Rollback Automation"
echo "========================================================"
echo "  Environment:        ${ENV}"
echo "  Component:          ${APP}"
echo "  Target Revision:    ${TARGET_REVISION}"
echo "  Target Traffic:     ${TRAFFIC}%"
echo "  Resource Group:     ${RESOURCE_GROUP}"
echo "  Dry Run Mode:       ${DRY_RUN}"
echo "  Verify After:       ${VERIFY}"
echo "========================================================"

rollback_app() {
    local APP_NAME="$1"
    echo -e "${BLUE}[INFO]${NC} Initiating traffic shift on ${APP_NAME} to revision ${TARGET_REVISION}..."

    if [ "${DRY_RUN}" = true ]; then
        echo -e "${YELLOW}[DRY-RUN]${NC} az containerapp revision set-traffic \\"
        echo -e "${YELLOW}[DRY-RUN]${NC}   --name ${APP_NAME} \\"
        echo -e "${YELLOW}[DRY-RUN]${NC}   --resource-group ${RESOURCE_GROUP} \\"
        echo -e "${YELLOW}[DRY-RUN]${NC}   --revision-weight ${TARGET_REVISION}=${TRAFFIC}"
        return 0
    fi

    # Check az CLI availability
    if ! command -v az >/dev/null 2>&1; then
        echo -e "${RED}[ERROR]${NC} Azure CLI ('az') not found in PATH." >&2
        return 1
    fi

    # Activate target revision if needed
    az containerapp revision activate \
        --name "${APP_NAME}" \
        --resource-group "${RESOURCE_GROUP}" \
        --revision "${TARGET_REVISION}" \
        --output none || true

    # Shift traffic to target revision
    az containerapp revision set-traffic \
        --name "${APP_NAME}" \
        --resource-group "${RESOURCE_GROUP}" \
        --revision-weight "${TARGET_REVISION}=${TRAFFIC}" \
        --output none
}

# Determine target container app names
case "${APP}" in
    api)
        rollback_app "ca-api-lensio-${ENV}"
        ;;
    dashboard)
        rollback_app "ca-dash-lensio-${ENV}"
        ;;
    all)
        rollback_app "ca-api-lensio-${ENV}"
        rollback_app "ca-dash-lensio-${ENV}"
        ;;
    *)
        echo -e "${RED}[ERROR]${NC} Invalid app '${APP}'. Must be api, dashboard, or all." >&2
        exit 1
        ;;
esac

END_TIME=$(date +%s)
ELAPSED_SEC=$(( END_TIME - START_TIME ))

echo -e "${GREEN}[SUCCESS]${NC} Traffic shift completed in ${ELAPSED_SEC}s (Target SLA: <60s)."

# Run post-rollback verification
if [ "${VERIFY}" = true ] && [ "${DRY_RUN}" = false ]; then
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    if [ -z "${API_URL}" ]; then
        if [ "${ENV}" = "production" ]; then
            API_URL="https://api.lensio.dev"
        else
            API_URL="https://api.staging.lensio.dev"
        fi
    fi
    echo -e "${BLUE}[INFO]${NC} Executing post-rollback verification against ${API_URL}..."
    "${SCRIPT_DIR}/smoke-test.sh" "${API_URL}" || {
        echo -e "${RED}[FAIL]${NC} Post-rollback verification failed!" >&2
        exit 1
    }
fi

echo "========================================================"
echo -e "${GREEN}✅ Rollback Procedure Succeeded in ${ELAPSED_SEC}s!${NC}"
echo "========================================================"
exit 0
