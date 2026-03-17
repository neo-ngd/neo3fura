# Neofura
Neofura is a service for quickly getting data from the [Neo](www.neo.org) , If you are a developer on neo and need performance or good productivity, you will love Neofura.

For more info, Please refer to [Neofura-Doc](https://neo-ngd.github.io/neo3fura/) .

## Quick Start

```
./start.sh staging
./start.sh test
./start.sh test2
./start.sh dev
```

Compose files:

```
docker compose -p mainnet -f docker-compose.mainnet.yml up -d --build
docker compose -p testnet -f docker-compose.testnet.yml up -d --build
docker compose -p test2 -f docker-compose.yml up -d --build
docker compose -p dev -f docker-compose.yml up -d --build
```

`./start.sh` now treats its single argument as the runtime. Supported values are `dev`, `test`, `test2`, and `staging`.

Compatibility aliases are also available:

- `./start.sh staging` maps to mainnet with `RUNTIME=staging`
- `./start.sh test` maps to testnet with `RUNTIME=test`
- `./start.sh mainnet` is the same as `./start.sh staging`
- `./start.sh testnet` is the same as `./start.sh test`

The two compose files publish the same host ports, so they are intended to be run one at a time on the same machine.

## API Examples

``
curl --location --request POST 'https://testneofura.ngd.network:444' \
--header 'Content-Type: application/json' \
--data-raw '{
"jsonrpc": "2.0",
"method": "GetBestBlockHash",
"params": {},
"id": 1
}'
``

The response will be 

``
{
"id": 1,
"result": {
"hash": "0x226844a595780dd2881bbfedbf4ffabe25fcc691969359aa1b7f87a715cdea75"
},
"error": null
}
``

## Swagger UI

After service startup, open:

``
http://127.0.0.1:1926/swagger/
``

OpenAPI source:

``
http://127.0.0.1:1926/swagger/openapi.yaml
``

## Feathers

### High Concurrency
Thanks to the native Go framework that the Neofura can handle more than  requests in a second.

### Auto Repost
If the service meet the method that have not been implemented the service will repost to the NEO node, which means, any NEO rpc request can be sent to Neofura.

### GraphQL(coming soon)
A graphql server will be implemented soon to have a more flexible query.

## Appreciations
We really appreciate all the partners in contributing codes in this project, especially [vikkko](https://github.com/vikkkko) and [joeqian](https://github.com/joeqian10/). Also, [WSbaikaishui](https://github.com/WSbaikaishui), [zifanwangsteven](https://github.com/zifanwangsteven), [RookieCoderrr](https://github.com/RookieCoderrr) come up with many advices. [Celia18305](https://github.com/Celia18305) is a perfect document worker who helps to make all the documents in order.
Don't forget to give us a STAR if you like it! 
