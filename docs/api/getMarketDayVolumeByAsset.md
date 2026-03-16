# GetMarketDayVolumeByAsset
Auto-generated draft for parameter confirmation.
<hr>

> PENDING_CONFIRMATION: please verify parameter names/types with implementation and business semantics.

### Parameters

| Name | Type | Description | Required |
| --- | --- | --- | --- |
| AssetHash | h160.T | TODO | Required |
| LastDays | int64 | TODO | Required |
| Raw | *map[string]interface{} | TODO | Optional |

### Example

Request body

```json
{
  "jsonrpc": "2.0",
  "method": "GetMarketDayVolumeByAsset",
  "params": {
    "AssetHash": "0xd2a4cff31913016155e38e474a2c06d08be276cf",
    "LastDays": 0,
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
