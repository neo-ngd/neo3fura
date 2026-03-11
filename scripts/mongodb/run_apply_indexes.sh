#!/usr/bin/env bash
set -euo pipefail

DB_NAME="${1:-${MONGO_DB_NAME:-}}"
RUNTIME_NAME="${RUNTIME:-test}"
MONGO_CONTAINER="${MONGO_CONTAINER:-mongo_${RUNTIME_NAME}}"
MONGO_SHELL="${MONGO_SHELL:-mongosh}"

if [[ -z "${DB_NAME}" ]]; then
  echo "Usage: $0 <db_name>"
  echo "Or set MONGO_DB_NAME=<db_name>"
  exit 1
fi

if ! docker ps --format '{{.Names}}' | grep -qx "${MONGO_CONTAINER}"; then
  echo "Mongo container not found: ${MONGO_CONTAINER}"
  echo "Tip: export RUNTIME=test|staging or set MONGO_CONTAINER explicitly."
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
APPLY_JS="${SCRIPT_DIR}/apply_indexes.js"

echo "[INFO] applying indexes to db=${DB_NAME} on container=${MONGO_CONTAINER}"
docker exec -i "${MONGO_CONTAINER}" "${MONGO_SHELL}" "${DB_NAME}" < "${APPLY_JS}"
echo "[DONE] apply finished"
