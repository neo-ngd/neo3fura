# GetNFTByAssetClassPrimaryMarket
Auto-generated draft for parameter confirmation.
<hr>

> PENDING_CONFIRMATION: please verify parameter names/types with implementation and business semantics.

### Parameters

| Name | Type | Description | Required |
| --- | --- | --- | --- |
| Asset | h160.T | TODO | Required |
| PrimaryMarket | h160.T | TODO | Required |
| Class | string | TODO | Required |
| ClassName | string | TODO | Required |
| Limit | int64 | TODO | Optional |
| Skip | int64 | TODO | Optional |
| Raw | *map[string]interface{} | TODO | Optional |

### Example

Request body

```json
{
  "jsonrpc": "2.0",
  "method": "GetNFTByAssetClassPrimaryMarket",
  "params": {
    "Asset": "",
    "PrimaryMarket": "",
    "Class": "",
    "ClassName": "",
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
