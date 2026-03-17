#!/bin/sh

set -eu

usage() {
    cat <<'EOF'
Usage:
  ./start.sh mainnet
  ./start.sh testnet

Legacy aliases:
  ./start.sh STAGING   # mainnet
  ./start.sh TEST      # testnet
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
    mainnet|MAINNET|STAGING|staging)
        PROJECT="mainnet"
        COMPOSE_FILE="docker-compose.mainnet.yml"
        RUNTIME="staging"
        ;;
    testnet|TESTNET|TEST|test)
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

$DOCKER_COMPOSE -p "$PROJECT" -f "$COMPOSE_FILE" down --remove-orphans
$DOCKER_COMPOSE -p "$PROJECT" -f "$COMPOSE_FILE" up -d --build
