# Pagination Policy

For all high-frequency list APIs, prefer cursor pagination.

## Recommended usage

1. First request:
```json
{
  "Limit": 50
}
```

2. Next request:
```json
{
  "Limit": 50,
  "Cursor": "<nextCursor from previous response>"
}
```

## Rules

- `Cursor` is preferred for new clients.
- `Skip` is kept only for backward compatibility.
- If both `Cursor` and `Skip` are provided, `Cursor` takes precedence.
- `nextCursor` is returned only when a next page exists.

## Why

- Avoid deep `skip` performance degradation.
- Improve latency stability on large datasets.
- Reduce timeout risk on high-traffic list endpoints.
