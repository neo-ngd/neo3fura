# GetInfoByNFTList
Auto-generated draft for parameter confirmation.
<hr>

> PENDING_CONFIRMATION: please verify parameter names/types with implementation and business semantics.

### Parameters

| Name | Type | Description | Required |
| --- | --- | --- | --- |
| NFT | []struct | TODO | Required |
| Asset | h160.T | TODO | Required |
| TokenId | strval.T | TODO | Required |
| Raw | *map[string]interface{} | TODO | Optional |

### Example

Request body

```json
{
  "jsonrpc": "2.0",
  "method": "GetInfoByNFTList",
  "params": {
    "NFT": "",
    "Asset": "",
    "TokenId": "1",
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
