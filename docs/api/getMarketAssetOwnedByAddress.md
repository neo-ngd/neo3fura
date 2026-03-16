# GetMarketAssetOwnedByAddress
Auto-generated draft for parameter confirmation.
<hr>

> PENDING_CONFIRMATION: please verify parameter names/types with implementation and business semantics.

### Parameters

| Name | Type | Description | Required |
| --- | --- | --- | --- |
| Address | h160.T | TODO | Required |
| MarketHash | h160.T | TODO | Required |
| Limit | int64 | TODO | Optional |
| Skip | int64 | TODO | Optional |
| Raw | *map[string]interface{} | TODO | Optional |

### Example

Request body

```json
{
  "jsonrpc": "2.0",
  "method": "GetMarketAssetOwnedByAddress",
  "params": {
    "Address": "0x0bf916d727c75f2e51e1ab2c476304513da59701",
    "MarketHash": "0xd2a4cff31913016155e38e474a2c06d08be276cf",
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
