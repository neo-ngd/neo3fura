#!/bin/sh

set -eu

usage() {
    cat <<'EOF'
Usage:
  ./start.sh <runtime>

Supported runtimes:
  dev | test | test2 | staging

Compatibility aliases:
  ./start.sh mainnet   # same as staging
  ./start.sh testnet   # same as test

Legacy aliases:
  ./start.sh STAGING
  ./start.sh TEST
EOF
}

if [ "$#" -ne 1 ]; then
    usage
    exit 1
fi

if docker compose version >/dev/null 2>&1; then
    DOCKER_COMPOSE="docker compose"
else
    DOCKER_COMPOSE="docker-compose"
fi

case "$1" in
    dev|DEV)
        PROJECT="dev"
        COMPOSE_FILE="docker-compose.yml"
        RUNTIME="dev"
        ;;
    test|TEST)
        PROJECT="testnet"
        COMPOSE_FILE="docker-compose.testnet.yml"
        RUNTIME="test"
        ;;
    test2|TEST2)
        PROJECT="test2"
        COMPOSE_FILE="docker-compose.yml"
        RUNTIME="test2"
        ;;
    staging|STAGING)
        PROJECT="mainnet"
        COMPOSE_FILE="docker-compose.mainnet.yml"
        RUNTIME="staging"
        ;;
    mainnet|MAINNET)
        PROJECT="mainnet"
        COMPOSE_FILE="docker-compose.mainnet.yml"
        RUNTIME="staging"
        ;;
    testnet|TESTNET)
        PROJECT="testnet"
        COMPOSE_FILE="docker-compose.testnet.yml"
        RUNTIME="test"
        ;;
    *)
        usage
        exit 1
        ;;
esac

export RUNTIME

echo "starting ${PROJECT} with ${COMPOSE_FILE} (RUNTIME=${RUNTIME})"

echo "recreating neofura_http and neofura_ws only; existing dependencies will be reused"

$DOCKER_COMPOSE -p "$PROJECT" -f "$COMPOSE_FILE" rm -sf neofura_http neofura_ws
$DOCKER_COMPOSE -p "$PROJECT" -f "$COMPOSE_FILE" up -d --build neofura_http neofura_ws
