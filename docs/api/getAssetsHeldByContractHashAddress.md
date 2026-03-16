# GetAssetsHeldByContractHashAddress

Gets the assets information by the given contract script hash and user's address.
<hr>

### Parameters

|    Name    | Type | Description | Required |
| ---------- | --- |    ------    | ----|
| ContractHash     | string| The contract script hash | Required|
| Address   | string| The user's address | Required|
| Limit    | int|  The number of items to return| Optional|
| Skip    | int|  The number of items to skip| Optional |
| Cursor | string| Cursor for keyset pagination (from previous response's nextCursor)| Optional|

### Example

Request body

```powershell
curl --location --request POST 'https://testneofura.ngd.network:444' \
--header 'Content-Type: application/json' \
--data-raw '{
  "jsonrpc": "2.0",
  "method": "GetAssetsHeldByContractHashAddress",
  "params": {"Address":"0xeba621d37ff117d9ce73c1579bf260aa779cb392","ContractHash":"0xd2a4cff31913016155e38e474a2c06d08be276cf"},
  "id": 1
}'
```

Request body (with cursor)

```powershell
curl --location --request POST 'https://testneofura.ngd.network:444' \
--header 'Content-Type: application/json' \
--data-raw '{
  "jsonrpc": "2.0",
  "method": "GetAssetsHeldByContractHashAddress",
  "params": {"Address":"0xeba621d37ff117d9ce73c1579bf260aa779cb392","ContractHash":"0xd2a4cff31913016155e38e474a2c06d08be276cf","Limit":2,"Cursor":"eyJmIjp7...}"},
  "id": 1
}'
```

Response body

```json5

{
  "id": 1,
  "result": {
    "result": [
      {
        "_id": "614bef0ea14111843551a804",
        "address": "0xeba621d37ff117d9ce73c1579bf260aa779cb392",
        "asset": "0xd2a4cff31913016155e38e474a2c06d08be276cf",
        "balance": "5099945401484000",
        "tokenid": ""
      }
    ],
    "totalCount": 1,
    "nextCursor": "eyJmIjp7...}}"
  },
  "error": null
}
```
