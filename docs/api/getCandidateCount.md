# GetCandidateCount

Gets the count of candidates

<hr>

### Parameters
None


### Example

Request body

```
curl --location --request POST 'https://testneofura.ngd.network:444' \
--header 'Content-Type: application/json' \
--data-raw '{
  "jsonrpc": "2.0",
  "method": "GetCandidateCount",
  "params": {},
  "id": 1
}'
```
Response body

```json5
{
    "id": 1,
        "result": {
        "total counts": 49
    },
    "error": null
}
```
