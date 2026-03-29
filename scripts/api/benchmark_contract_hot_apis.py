#!/usr/bin/env python3
import json
import math
import os
import ssl
import statistics
import sys
import time
import urllib.error
import urllib.request
from datetime import datetime, timezone


DEFAULT_METHODS = [
    "GetNotificationByContractHash",
    "GetNep17TransferByContractHash",
]

DEFAULT_SKIPS = [0, 1000, 10000, 50000, 100000, 500000, 1000000]


def getenv_int(name, default):
    raw = os.getenv(name, "").strip()
    if not raw:
        return default
    try:
        return int(raw)
    except ValueError:
        print(f"[FATAL] invalid integer for {name}: {raw}", file=sys.stderr)
        sys.exit(1)


def getenv_list_int(name, default):
    raw = os.getenv(name, "").strip()
    if not raw:
        return default
    values = []
    for part in raw.split(","):
        item = part.strip()
        if not item:
            continue
        try:
            values.append(int(item))
        except ValueError:
            print(f"[FATAL] invalid integer in {name}: {item}", file=sys.stderr)
            sys.exit(1)
    return values or default


def getenv_list_str(name, default):
    raw = os.getenv(name, "").strip()
    if not raw:
        return default
    values = [item.strip() for item in raw.split(",") if item.strip()]
    return values or default


def rpc_call(rpc_url, method, params, timeout_sec):
    payload = json.dumps(
        {
            "jsonrpc": "2.0",
            "method": method,
            "params": params,
            "id": 1,
        }
    ).encode("utf-8")
    req = urllib.request.Request(
        rpc_url,
        data=payload,
        headers={"Content-Type": "application/json"},
        method="POST",
    )

    started = time.perf_counter()
    insecure_tls = os.getenv("INSECURE_TLS", "").strip().lower() in {"1", "true", "yes"}
    ssl_context = None
    if insecure_tls:
        ssl_context = ssl._create_unverified_context()

    try:
        with urllib.request.urlopen(req, timeout=timeout_sec, context=ssl_context) as resp:
            body = resp.read()
            elapsed_ms = (time.perf_counter() - started) * 1000.0
            text = body.decode("utf-8", errors="replace")
            try:
                parsed = json.loads(text)
            except json.JSONDecodeError as exc:
                return {
                    "ok": False,
                    "elapsed_ms": elapsed_ms,
                    "error": f"invalid_json: {exc}",
                    "status": getattr(resp, "status", 200),
                    "body": text,
                }
            if parsed.get("error") is not None:
                return {
                    "ok": False,
                    "elapsed_ms": elapsed_ms,
                    "error": parsed.get("error"),
                    "status": getattr(resp, "status", 200),
                    "body": parsed,
                }
            return {
                "ok": True,
                "elapsed_ms": elapsed_ms,
                "status": getattr(resp, "status", 200),
                "body": parsed,
            }
    except urllib.error.HTTPError as exc:
        elapsed_ms = (time.perf_counter() - started) * 1000.0
        body = exc.read().decode("utf-8", errors="replace")
        return {
            "ok": False,
            "elapsed_ms": elapsed_ms,
            "error": f"http_{exc.code}",
            "status": exc.code,
            "body": body,
        }
    except Exception as exc:
        elapsed_ms = (time.perf_counter() - started) * 1000.0
        return {
            "ok": False,
            "elapsed_ms": elapsed_ms,
            "error": str(exc),
            "status": None,
            "body": None,
        }


def extract_next_cursor(resp):
    if not resp or not resp.get("ok"):
        return ""
    body = resp.get("body") or {}
    result = body.get("result") or {}
    if not isinstance(result, dict):
        return ""
    cursor = result.get("nextCursor")
    return cursor if isinstance(cursor, str) else ""


def build_cursor_for_offset(rpc_url, method, contract_hash, limit, skip, timeout_sec):
    if skip <= 0:
        return {"ok": True, "cursor": "", "steps": 0}
    if limit <= 0:
        return {"ok": False, "error": "limit_must_be_positive"}
    if skip % limit != 0:
        return {
            "ok": False,
            "error": f"skip_{skip}_is_not_divisible_by_limit_{limit}",
        }

    steps = skip // limit
    cursor = ""
    for step in range(steps):
        params = {
            "ContractHash": contract_hash,
            "Limit": limit,
        }
        if cursor:
            params["Cursor"] = cursor
        resp = rpc_call(rpc_url, method, params, timeout_sec)
        if not resp["ok"]:
            return {
                "ok": False,
                "error": f"cursor_walk_failed_at_step_{step + 1}: {resp['error']}",
            }
        cursor = extract_next_cursor(resp)
        if not cursor:
            return {
                "ok": False,
                "error": f"cursor_exhausted_at_step_{step + 1}",
            }
    return {"ok": True, "cursor": cursor, "steps": steps}


def safe_round(value):
    return round(value, 2) if value is not None else "-"


def summarize_runs(runs):
    successes = [run["elapsed_ms"] for run in runs if run["ok"]]
    fail_count = len(runs) - len(successes)
    success_rate = (len(successes) / len(runs)) if runs else 0.0
    avg_ms = statistics.mean(successes) if successes else None
    median_ms = statistics.median(successes) if successes else None
    p95_ms = None
    if successes:
        ordered = sorted(successes)
        index = max(0, math.ceil(0.95 * len(ordered)) - 1)
        p95_ms = ordered[index]
    last_error = None
    for run in reversed(runs):
        if not run["ok"]:
            last_error = run["error"]
            break
    return {
        "avg_ms": avg_ms,
        "median_ms": median_ms,
        "p95_ms": p95_ms,
        "success_rate": success_rate,
        "fail_count": fail_count,
        "last_error": last_error,
    }


def render_table(rows):
    lines = [
        "| Rank | method | avgMs | medianMs | p95Ms | successRate | failCount | lastError |",
        "|---:|---|---:|---:|---:|---:|---:|---|",
    ]
    for idx, row in enumerate(rows, start=1):
        lines.append(
            "| {rank} | {method} | {avg} | {median} | {p95} | {success_rate} | {fail_count} | {last_error} |".format(
                rank=idx,
                method=row["method"],
                avg=safe_round(row["avg_ms"]),
                median=safe_round(row["median_ms"]),
                p95=safe_round(row["p95_ms"]),
                success_rate=safe_round(row["success_rate"]),
                fail_count=row["fail_count"],
                last_error=row["last_error"] if row["last_error"] is not None else "",
            )
        )
    return "\n".join(lines)


def main():
    rpc_url = os.getenv("RPC_URL", "http://127.0.0.1:1926").strip()
    contract_hash = os.getenv("CONTRACT_HASH", "").strip()
    limit = getenv_int("LIMIT", 20)
    repeats = getenv_int("REPEATS", 3)
    timeout_sec = getenv_int("TIMEOUT_SEC", 30)
    skips = getenv_list_int("SKIPS", DEFAULT_SKIPS)
    methods = getenv_list_str("METHODS", DEFAULT_METHODS)
    pause_ms = getenv_int("PAUSE_MS", 0)

    if not contract_hash:
        print("[FATAL] CONTRACT_HASH is required", file=sys.stderr)
        sys.exit(1)

    report_lines = [
        "# Contract Hot API Benchmark",
        "",
        f"- GeneratedAt: {datetime.now(timezone.utc).isoformat()}",
        f"- RPC_URL: {rpc_url}",
        f"- ContractHash: {contract_hash}",
        f"- Methods: {', '.join(methods)}",
        f"- Limit: {limit}",
        f"- Repeats: {repeats}",
        f"- TimeoutSec: {timeout_sec}",
        f"- Skips: {', '.join(str(v) for v in skips)}",
        "- Paging: skip=0 uses first page; skip>0 uses cursor walk then cursor request",
    ]

    cursor_cache = {}

    for skip in skips:
        grouped = []
        for method in methods:
            runs = []
            cursor_key = (method, skip)
            cursor_info = {"ok": True, "cursor": "", "steps": 0}
            if skip > 0:
                if cursor_key not in cursor_cache:
                    cursor_cache[cursor_key] = build_cursor_for_offset(
                        rpc_url, method, contract_hash, limit, skip, timeout_sec
                    )
                cursor_info = cursor_cache[cursor_key]

            for _ in range(repeats):
                if not cursor_info["ok"]:
                    runs.append(
                        {
                            "ok": False,
                            "elapsed_ms": None,
                            "error": cursor_info["error"],
                            "status": None,
                            "body": None,
                        }
                    )
                else:
                    params = {
                        "ContractHash": contract_hash,
                        "Limit": limit,
                    }
                    if skip > 0:
                        params["Cursor"] = cursor_info["cursor"]
                    else:
                        params["Skip"] = 0
                    runs.append(rpc_call(rpc_url, method, params, timeout_sec))
                if pause_ms > 0:
                    time.sleep(pause_ms / 1000.0)
            summary = summarize_runs(runs)
            summary["method"] = method
            summary["paging"] = "cursor" if skip > 0 else "skip"
            grouped.append(summary)

        grouped.sort(
            key=lambda item: (
                item["avg_ms"] is None,
                float("inf") if item["avg_ms"] is None else -item["avg_ms"],
            )
        )

        report_lines.extend(
            [
                "",
                f"## Skip = {skip}",
                "",
                render_table(grouped),
            ]
        )

    print("\n".join(report_lines))


if __name__ == "__main__":
    main()
