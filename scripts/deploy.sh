#!/usr/bin/env bash
# ==============================================================================
# Lensio Azure Container Apps Deploy
# ==============================================================================
# Rolls one image tag out to the API and dashboard of an environment.
# Leaves at most two active revisions per app: the new one serving 100%, and the
# one it replaced, still warm so scripts/rollback.sh can shift back in seconds.
# ==============================================================================

set -euo pipefail

ENV="${1:?usage: deploy.sh <staging|production> <image-tag>}"
TAG="${2:?usage: deploy.sh <staging|production> <image-tag>}"
REGISTRY="${REGISTRY:-ghcr.io/amirfaisalz/lensio}"
RESOURCE_GROUP="rg-lensio-${ENV}"

case "${ENV}" in
    staging|production) ;;
    *) echo "[ERROR] env must be staging or production, got '${ENV}'" >&2; exit 1 ;;
esac

deploy_app() {
    local app="$1" image="$2" rev

    # Keep only the revision serving traffic warm; it becomes the rollback target.
    for rev in $(az containerapp revision list --name "${app}" --resource-group "${RESOURCE_GROUP}" \
        --query "[?properties.active && properties.trafficWeight==\`0\`].name" -o tsv); do
        echo "[INFO] ${app}: deactivating idle revision ${rev}"
        az containerapp revision deactivate --name "${app}" --resource-group "${RESOURCE_GROUP}" --revision "${rev}" --output none
    done

    echo "[INFO] ${app}: deploying ${image}"
    az containerapp update --name "${app}" --resource-group "${RESOURCE_GROUP}" --image "${image}" --output none

    # A revision that fails to boot leaves traffic on the old one, so smoke tests
    # pass against the previous build. Fail the deploy until the new one is ready.
    local latest ready i
    for i in $(seq 1 "${READY_TIMEOUT_SEC:-180}"); do
        read -r latest ready < <(az containerapp show --name "${app}" --resource-group "${RESOURCE_GROUP}" \
            --query "[properties.latestRevisionName, properties.latestReadyRevisionName]" -o tsv | tr '\n' ' ')
        [ "${latest}" = "${ready}" ] && break
        if [ "${i}" = "${READY_TIMEOUT_SEC:-180}" ]; then
            echo "[ERROR] ${app}: revision ${latest} never became ready (last ready: ${ready}); traffic left untouched" >&2
            exit 1
        fi
        sleep 1
    done

    # A rollback pins traffic to a named revision; point it back at the latest.
    az containerapp ingress traffic set --name "${app}" --resource-group "${RESOURCE_GROUP}" \
        --revision-weight latest=100 --output none
}

deploy_app "ca-api-lensio-${ENV}" "${REGISTRY}/lensio-api:${TAG}"
deploy_app "ca-dash-lensio-${ENV}" "${REGISTRY}/lensio-dashboard:${TAG}"
echo "[SUCCESS] ${ENV} now serving ${TAG}"
