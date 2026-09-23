#!/bin/sh
# Writes /config.js from API_URL at container start, so one dashboard image can
# be promoted from staging to production unchanged. Run by the nginx image's
# /docker-entrypoint.sh before nginx starts.
set -eu

API_URL="${API_URL:-}"
CONFIG_PATH="${LENSIO_CONFIG_PATH:-/usr/share/nginx/html/config.js}"

# The value is written into JavaScript the browser executes: accept a bare
# origin only, so nothing can break out of the string literal. The character
# check comes first because grep matches per line, so a value with a newline
# would pass on its first line alone.
bad_origin() {
    echo "40-lensio-config: API_URL must be an origin like https://api.example.com, got '${API_URL}'" >&2
    exit 1
}
if [ -n "${API_URL}" ]; then
    case "${API_URL}" in
        *[!A-Za-z0-9.:/-]*) bad_origin ;;
    esac
    printf '%s' "${API_URL}" | grep -Eq '^https?://[A-Za-z0-9.-]+(:[0-9]{1,5})?$' || bad_origin
fi

printf 'window.__LENSIO_CONFIG__ = { apiUrl: "%s" };\n' "${API_URL}" > "${CONFIG_PATH}"
echo "40-lensio-config: API origin set to '${API_URL:-same-origin}'"
