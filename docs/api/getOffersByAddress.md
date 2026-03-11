# GetOffersByAddress
Auto-generated draft for parameter confirmation.
<hr>

> PENDING_CONFIRMATION: please verify parameter names/types with implementation and business semantics.

### Parameters

| Name | Type | Description | Required |
| --- | --- | --- | --- |
| Address | h160.T | TODO | Required |
| OfferState | strval.T | TODO | Required |
| Limit | int64 | TODO | Optional |
| Skip | int64 | TODO | Optional |

### Example

Request body

```json
{
  "jsonrpc": "2.0",
  "method": "GetOffersByAddress",
  "params": {
    "Address": "0x0bf916d727c75f2e51e1ab2c476304513da59701",
    "OfferState": "",
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
