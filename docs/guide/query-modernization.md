# Query Modernization Guide

This project stores parsed blockchain metadata in MongoDB. As data grows, full scans and deep pagination become the main bottlenecks.  
This guide describes a practical upgrade path that keeps current APIs compatible.

## 1. Changes already applied in code

- `neo3fura_http/lib/cli/src.go`
  - Unified `limit/skip` normalization in query helpers.
  - Added query timeout (`NEO3FURA_QUERY_TIMEOUT_MS`, default `15000` ms).
  - Added runtime-configurable page limits:
    - `NEO3FURA_DEFAULT_LIMIT` (fallback to current code default)
    - `NEO3FURA_MAX_LIMIT` (fallback to current code max)
  - Improved `$limit` normalization for aggregation pipelines (supports `int/int32/int64/float/string` inputs).
  - Added safer cursor close handling (warn log instead of process-kill).
  - For empty filter counts, uses `EstimatedDocumentCount` first to reduce count overhead.
- `neo3fura_http/biz/api/src.GetAddressList.go`
  - Moved `$skip/$limit` before heavy `$lookup` stages (page-first strategy).
- `neo3fura_http/biz/api/src.GetNFTByWords.go`
  - Reordered lookup pipeline to match `asset+tokenid` first, then keyword filter.
  - Added regex escaping for input words.
  - Added `$limit: 1` inside lookup to reduce memory.
  - Replaced full fetch + `len()` counting with aggregation `$count`.

## 2. Index plan (MongoDB)

Create/verify these indexes first:

```javascript
db.Transaction.createIndex({ blocktime: -1 })
db.Address.createIndex({ firstusetime: -1 })
db["Address-Asset"].createIndex({ address: 1, asset: 1 })
db.TransferNotification.createIndex({ from: 1, to: 1 })
db.Nep11TransferNotification.createIndex({ from: 1, to: 1 })
db.Market.createIndex({ asset: 1, market: 1, amount: 1, tokenid: 1 })
db.Nep11Properties.createIndex({ asset: 1, tokenid: 1 })
```

Run with project scripts (recommended):

```bash
# example
export RUNTIME=test
./scripts/mongodb/run_apply_indexes.sh neo3fura_test
./scripts/mongodb/run_verify_indexes.sh neo3fura_test
```

If your Mongo container name or shell binary differs:

```bash
MONGO_CONTAINER=my-mongo MONGO_SHELL=mongo ./scripts/mongodb/run_apply_indexes.sh <db_name>
```

For `GetNFTByWords`, if you keep `properties` as JSON string, regex performance is inherently limited.  
For sustainable search, store normalized fields (for example `name`, `series`, `traits`) as separate keys.

## 3. Search engine migration path

Recommended strategy: keep MongoDB as source of truth, add search index as read-optimized sidecar.

1. Add a denormalized search document per NFT (`asset`, `tokenid`, `name`, `collection`, traits, status, timestamps).
2. Build async indexer from block parsing events (or MongoDB change stream).
3. Dual-read rollout:
   - Phase A: keep Mongo query as primary, compare search-engine result in shadow mode.
   - Phase B: switch keyword/search endpoints to search engine.
   - Phase C: keep Mongo fallback and health-based failover.
4. Backfill historical data once, then incremental sync.

Atlas Search is fastest to adopt if you already run on MongoDB Atlas.  
OpenSearch/Elasticsearch is better when you need advanced ranking, custom analyzers, and cross-domain search workloads.

## 4. API compatibility guidance

- Keep existing `limit/skip` params for backward compatibility.
- For very deep pages, add cursor-based pagination (`nextCursor`) in new API methods.
- Keep `totalCount` optional for expensive queries (`includeTotal=false` on new methods).

## 5. Observability checklist

- Track per-method p95/p99 latency.
- Log slow queries with method name, collection, and duration.
- Add dashboard split by:
  - full scan count,
  - aggregation duration,
  - timeout count,
  - top N slow endpoints.
