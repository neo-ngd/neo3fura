# GetBridgeTxByNonce
Auto-generated draft for parameter confirmation.
<hr>

> PENDING_CONFIRMATION: please verify parameter names/types with implementation and business semantics.

### Parameters

| Name | Type | Description | Required |
| --- | --- | --- | --- |
| ContractHash | h160.T | TODO | Required |
| TokenHash | h160.T | TODO | Required |
| Nonce | int64 | TODO | Required |
| Limit | int64 | TODO | Optional |
| Skip | int64 | TODO | Optional |

### Example

Request body

```json
{
  "jsonrpc": "2.0",
  "method": "GetBridgeTxByNonce",
  "params": {
    "ContractHash": "0xd2a4cff31913016155e38e474a2c06d08be276cf",
    "TokenHash": "0xd2a4cff31913016155e38e474a2c06d08be276cf",
    "Nonce": 0,
    "Limit": 0,
    "Skip": 0
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
