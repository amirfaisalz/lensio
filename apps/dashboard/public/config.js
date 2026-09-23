// Dev default: same-origin, which the Vite dev server proxies to the local API.
// The dashboard container overwrites this file at start from API_URL
// (docker-entrypoint.d/40-lensio-config.sh).
window.__LENSIO_CONFIG__ = { apiUrl: "" };
