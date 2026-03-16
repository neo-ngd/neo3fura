#!/usr/bin/env bash
set -u

RPC_URL="${RPC_URL:-http://127.0.0.1:1926}"
CURL_TIMEOUT="${CURL_TIMEOUT:-20}"

if ! command -v curl >/dev/null 2>&1; then
  echo "[FATAL] curl not found"
  exit 1
fi

HAS_JQ=0
if command -v jq >/dev/null 2>&1; then
  HAS_JQ=1
fi

HAS_PYTHON3=0
if command -v python3 >/dev/null 2>&1; then
  HAS_PYTHON3=1
fi

if [[ "${HAS_JQ}" -eq 0 && "${HAS_PYTHON3}" -eq 0 ]]; then
  echo "[FATAL] neither jq nor python3 found"
  exit 1
fi

PASS_COUNT=0
FAIL_COUNT=0
SKIP_COUNT=0

print_sep() {
  echo "------------------------------------------------------------"
}

rpc_call() {
  local method="$1"
  local params_json="$2"
  curl -sS --max-time "${CURL_TIMEOUT}" \
    -H "Content-Type: application/json" \
    -X POST "${RPC_URL}" \
    -d "{\"jsonrpc\":\"2.0\",\"method\":\"${method}\",\"params\":${params_json},\"id\":1}"
}

json_error() {
  local resp="$1"
  if [[ -z "${resp}" ]]; then
    echo "empty_response"
    return 0
  fi
  if [[ "${HAS_JQ}" -eq 1 ]]; then
    echo "${resp}" | jq -r '.error'
    return 0
  fi
  printf '%s' "${resp}" | python3 -c 'import json,sys; s=sys.stdin.read(); 
try:
 d=json.loads(s)
except Exception:
 print("invalid_json"); raise SystemExit(0)
v=d.get("error", None); print("null" if v is None else str(v))'
}

json_next_cursor() {
  local resp="$1"
  if [[ -z "${resp}" ]]; then
    echo ""
    return 0
  fi
  if [[ "${HAS_JQ}" -eq 1 ]]; then
    echo "${resp}" | jq -r '.result.nextCursor // empty'
    return 0
  fi
  printf '%s' "${resp}" | python3 -c 'import json,sys; s=sys.stdin.read();
try:
 d=json.loads(s)
except Exception:
 print(""); raise SystemExit(0)
r=d.get("result",{}) or {}; print(r.get("nextCursor","") or "")'
}

json_first_address() {
  local resp="$1"
  if [[ -z "${resp}" ]]; then
    echo ""
    return 0
  fi
  if [[ "${HAS_JQ}" -eq 1 ]]; then
    echo "${resp}" | jq -r '.result.result[0].address // empty'
    return 0
  fi
  printf '%s' "${resp}" | python3 -c 'import json,sys; s=sys.stdin.read();
try:
 d=json.loads(s)
except Exception:
 print(""); raise SystemExit(0)
r=d.get("result",{}) or {}; arr=r.get("result",[]) or []; print((arr[0].get("address","") if arr and isinstance(arr[0],dict) else "") or "")'
}

assert_success() {
  local name="$1"
  local resp="$2"
  local err_val
  err_val="$(json_error "${resp}")"
  if [[ "${err_val}" != "null" ]]; then
    echo "[FAIL] ${name}: error=${err_val}"
    echo "       response=${resp}"
    FAIL_COUNT=$((FAIL_COUNT + 1))
    return 1
  fi
  echo "[PASS] ${name}"
  PASS_COUNT=$((PASS_COUNT + 1))
  return 0
}

run_and_check() {
  local name="$1"
  local method="$2"
  local params="$3"
  local __resultvar="$4"
  local resp
  if ! resp="$(rpc_call "${method}" "${params}")"; then
    echo "[FAIL] ${name}: request failed (RPC unavailable or timeout)"
    FAIL_COUNT=$((FAIL_COUNT + 1))
    printf -v "${__resultvar}" '%s' ""
    return 1
  fi
  if ! assert_success "${name}" "${resp}"; then
    printf -v "${__resultvar}" '%s' ""
    return 1
  fi
  printf -v "${__resultvar}" '%s' "${resp}"
  return 0
}

extract_next_cursor() {
  local resp="$1"
  json_next_cursor "${resp}"
}

extract_first_address() {
  local resp="$1"
  json_first_address "${resp}"
}

print_sep
echo "[INFO] RPC_URL=${RPC_URL}"
print_sep

# 1) GetTransactionList + cursor
tx_resp=""
run_and_check "GetTransactionList page1" "GetTransactionList" '{"Limit":2}' tx_resp
tx_cursor="$(extract_next_cursor "${tx_resp}")"
if [[ -n "${tx_cursor}" ]]; then
  run_and_check "GetTransactionList page2(cursor)" "GetTransactionList" "{\"Limit\":2,\"Cursor\":\"${tx_cursor}\"}" tx_resp2 || true
else
  echo "[SKIP] GetTransactionList page2(cursor): no nextCursor"
  SKIP_COUNT=$((SKIP_COUNT + 1))
fi

# 2) GetAddressList + cursor
addr_list_resp=""
run_and_check "GetAddressList page1" "GetAddressList" '{"Limit":2}' addr_list_resp
addr_cursor="$(extract_next_cursor "${addr_list_resp}")"
if [[ -n "${addr_cursor}" ]]; then
  run_and_check "GetAddressList page2(cursor)" "GetAddressList" "{\"Limit\":2,\"Cursor\":\"${addr_cursor}\"}" addr_list_resp2 || true
else
  echo "[SKIP] GetAddressList page2(cursor): no nextCursor"
  SKIP_COUNT=$((SKIP_COUNT + 1))
fi

address="$(extract_first_address "${addr_list_resp}")"
if [[ -z "${address}" ]]; then
  echo "[SKIP] address-based APIs: no address from GetAddressList result"
  SKIP_COUNT=$((SKIP_COUNT + 4))
else
  echo "[INFO] sample address=${address}"

  # 3) GetRawTransactionByAddress + cursor
  raw_resp=""
  run_and_check "GetRawTransactionByAddress page1" "GetRawTransactionByAddress" "{\"Address\":\"${address}\",\"Limit\":2}" raw_resp || true
  raw_cursor="$(extract_next_cursor "${raw_resp}")"
  if [[ -n "${raw_cursor}" ]]; then
    run_and_check "GetRawTransactionByAddress page2(cursor)" "GetRawTransactionByAddress" "{\"Address\":\"${address}\",\"Limit\":2,\"Cursor\":\"${raw_cursor}\"}" raw_resp2 || true
  else
    echo "[SKIP] GetRawTransactionByAddress page2(cursor): no nextCursor"
    SKIP_COUNT=$((SKIP_COUNT + 1))
  fi

  # 4) GetNep17TransferByAddress + cursor
  n17_resp=""
  run_and_check "GetNep17TransferByAddress page1" "GetNep17TransferByAddress" "{\"Address\":\"${address}\",\"Limit\":2,\"ExcludeBonusAndBurn\":true}" n17_resp || true
  n17_cursor="$(extract_next_cursor "${n17_resp}")"
  if [[ -n "${n17_cursor}" ]]; then
    run_and_check "GetNep17TransferByAddress page2(cursor)" "GetNep17TransferByAddress" "{\"Address\":\"${address}\",\"Limit\":2,\"ExcludeBonusAndBurn\":true,\"Cursor\":\"${n17_cursor}\"}" n17_resp2 || true
  else
    echo "[SKIP] GetNep17TransferByAddress page2(cursor): no nextCursor"
    SKIP_COUNT=$((SKIP_COUNT + 1))
  fi

  # 5) GetNep11TransferByAddress + cursor
  n11_resp=""
  run_and_check "GetNep11TransferByAddress page1" "GetNep11TransferByAddress" "{\"Address\":\"${address}\",\"Limit\":2}" n11_resp || true
  n11_cursor="$(extract_next_cursor "${n11_resp}")"
  if [[ -n "${n11_cursor}" ]]; then
    run_and_check "GetNep11TransferByAddress page2(cursor)" "GetNep11TransferByAddress" "{\"Address\":\"${address}\",\"Limit\":2,\"Cursor\":\"${n11_cursor}\"}" n11_resp2 || true
  else
    echo "[SKIP] GetNep11TransferByAddress page2(cursor): no nextCursor"
    SKIP_COUNT=$((SKIP_COUNT + 1))
  fi

  # 6) GetTransferByAddress + cursor
  tf_resp=""
  run_and_check "GetTransferByAddress page1" "GetTransferByAddress" "{\"Address\":\"${address}\",\"Limit\":2}" tf_resp || true
  tf_cursor="$(extract_next_cursor "${tf_resp}")"
  if [[ -n "${tf_cursor}" ]]; then
    run_and_check "GetTransferByAddress page2(cursor)" "GetTransferByAddress" "{\"Address\":\"${address}\",\"Limit\":2,\"Cursor\":\"${tf_cursor}\"}" tf_resp2 || true
  else
    echo "[SKIP] GetTransferByAddress page2(cursor): no nextCursor"
    SKIP_COUNT=$((SKIP_COUNT + 1))
  fi
fi

print_sep
echo "[SUMMARY] PASS=${PASS_COUNT} FAIL=${FAIL_COUNT} SKIP=${SKIP_COUNT}"
print_sep

if [[ "${FAIL_COUNT}" -gt 0 ]]; then
  exit 1
fi

exit 0
