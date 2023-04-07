# GetBidInfoByNFT
Gets the NFT bid information. 
<hr>

### Parameters

|    Name    | Type | Description | Required |
| ---------- | --- |    ------    | ----|
| Address     | string|  The user's address|Required|
| AssetHash     | string| The asset script hash |Optional|
| TokenId     | string| The NFT token ID | Optional |
| MarketHash     | string| The marketplace hash | Optional |

### Example

Request body

```powershell
curl --location --request GET 'https://testneofura.ngd.network:444' \
--header 'Content-Type: text/plain' \
--data-raw '{
  "jsonrpc": "2.0",
  "method": "GetBidInfoByNFT",
  "params": {
      "Address":"",
      "AssetHash": "0xc7b11b46f97bda7a8c82793841abba120e96695b",
      "TokenId":"az2dNYa7xEzk2XAQoHnH22k6AbO5/RkyqMDK64VuuXE=",
      "MarketHash":""
  
  },
  "id": 1
}'
```
Response body

```json
{
  "id": 1,
  "result": {
    "result": [
      {
        "asset": "0xc7b11b46f97bda7a8c82793841abba120e96695b",
        "auctionAsset": "0xd2a4cff31913016155e38e474a2c06d08be276cf",
        "bidAmount": "20",
        "bidder": "0x6fd49ab2f14a6bd9a060bb91fdbf29799a885a9e",
        "timestamp": 1639393572133,
        "tokenid": "az2dNYa7xEzk2XAQoHnH22k6AbO5/RkyqMDK64VuuXE="
      },
      ......
    ],
    "totalCount": 3
  },
  "error": null
}
```
