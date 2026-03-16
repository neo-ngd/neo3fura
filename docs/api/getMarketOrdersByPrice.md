# GetMarketOrdersByPrice
Auto-generated draft for parameter confirmation.
<hr>

> PENDING_CONFIRMATION: please verify parameter names/types with implementation and business semantics.

### Parameters

| Name | Type | Description | Required |
| --- | --- | --- | --- |
| AssetHash | h160.T | TODO | Required |
| MarketHash | h160.T | TODO | Required |
| Token | h160.T | TODO | Required |
| MinAmount | *big.Int | TODO | Required |
| MaxAmount | *big.Int | TODO | Required |
| Limit | int64 | TODO | Optional |
| Skip | int64 | TODO | Optional |
| Raw | *map[string]interface{} | TODO | Optional |

### Example

Request body

```json
{
  "jsonrpc": "2.0",
  "method": "GetMarketOrdersByPrice",
  "params": {
    "AssetHash": "0xd2a4cff31913016155e38e474a2c06d08be276cf",
    "MarketHash": "0xd2a4cff31913016155e38e474a2c06d08be276cf",
    "Token": "",
    "MinAmount": 0,
    "MaxAmount": 0,
    "Limit": 0,
    "Skip": 0,
    "Raw": {}
  },
  "id": 1
}
```

### Response

```json
{
  "id": 1,
  "result": {},
  "error": null
}
```
