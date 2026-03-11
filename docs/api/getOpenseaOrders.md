# GetOpenseaOrders
Auto-generated draft for parameter confirmation.
<hr>

> PENDING_CONFIRMATION: please verify parameter names/types with implementation and business semantics.

### Parameters

| Name | Type | Description | Required |
| --- | --- | --- | --- |
| AssetContractAddress | h160.T | TODO | Required |
| PaymentTokenAddress | h160.T | TODO | Required |
| Marker | string | TODO | Required |
| Taker | string | TODO | Required |
| Owner | string | TODO | Required |
| IsEnlish | string | TODO | Required |
| Bundled | bool | TODO | Required |
| IncludeBundled | bool | TODO | Required |
| ListedAfter | string | TODO | Required |
| ListedBefore | string | TODO | Required |
| TokenId | string | TODO | Required |
| TokenIds | []string | TODO | Required |
| Side | string | TODO | Required |
| SaleKind | string | TODO | Required |
| Limit | int64 | TODO | Optional |
| Offset | int64 | TODO | Required |
| OrderBy | string | TODO | Required |
| OrderDirection | string | TODO | Required |
| ApiKey | string | TODO | Required |

### Example

Request body

```json
{
  "jsonrpc": "2.0",
  "method": "GetOpenseaOrders",
  "params": {
    "AssetContractAddress": "0x0bf916d727c75f2e51e1ab2c476304513da59701",
    "PaymentTokenAddress": "0x0bf916d727c75f2e51e1ab2c476304513da59701",
    "Marker": "",
    "Taker": "",
    "Owner": "",
    "IsEnlish": "",
    "Bundled": false,
    "IncludeBundled": false,
    "ListedAfter": "",
    "ListedBefore": "",
    "TokenId": "1",
    "TokenIds": "1",
    "Side": "",
    "SaleKind": "",
    "Limit": 0,
    "Offset": 0,
    "OrderBy": "",
    "OrderDirection": "",
    "ApiKey": ""
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
