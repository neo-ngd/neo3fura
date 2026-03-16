# GetNFTList
Auto-generated draft for parameter confirmation.
<hr>

> PENDING_CONFIRMATION: please verify parameter names/types with implementation and business semantics.

### Parameters

| Name | Type | Description | Required |
| --- | --- | --- | --- |
| SecondaryMarket | h160.T | TODO | Required |
| PrimaryMarket | h160.T | TODO | Required |
| ContractHash | h160.T | TODO | Required |
| NFTState | strval.T | TODO | Required |
| Sort | strval.T | TODO | Required |
| Order | int64 | TODO | Optional |
| Limit | int64 | TODO | Optional |
| Skip | int64 | TODO | Optional |
| Raw | *map[string]interface{} | TODO | Optional |

### Example

Request body

```json
{
  "jsonrpc": "2.0",
  "method": "GetNFTList",
  "params": {
    "SecondaryMarket": "",
    "PrimaryMarket": "",
    "ContractHash": "0xd2a4cff31913016155e38e474a2c06d08be276cf",
    "NFTState": "",
    "Sort": "",
    "Order": 0,
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
