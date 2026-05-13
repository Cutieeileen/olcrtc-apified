#!/bin/sh
set -eu

# API mode: check the HTTP endpoint.
if [ "${OLCRTC_MODE:-srv}" = "api" ]; then
    port="${OLCRTC_API_LISTEN:-:8080}"
    port="${port##*:}"
    wget -q -O /dev/null "http://127.0.0.1:${port}/api/v1/status" \
        --header="Authorization: Bearer ${OLCRTC_MASTER_KEY:-}" 2>/dev/null
    exit $?
fi

# Default: check that the olcrtc process is running.
exe="$(readlink /proc/1/exe 2>/dev/null || true)"
case "$exe" in
    */olcrtc) exit 0 ;;
    *) exit 1 ;;
esac
