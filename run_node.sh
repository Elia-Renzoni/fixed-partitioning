#!/bin/bash

set -e

SUBNET="${SUBNET:-192.168.100}"
CONFIG_FILE="${CONFIG_FILE:-etc/config.yml}"
CONTAINER_NAME="${CONTAINER_NAME:-fixed-partitioning-$(date +%s)}"
IMAGE_NAME="${IMAGE_NAME:-localhost/project}"
NETWORK_NAME="${NETWORK_NAME:-mynet}"
PODMAN_BIN="${PODMAN_BIN:-podman}"

generate_ip() {
    local octet=$((RANDOM % 254 + 1))
    echo "$SUBNET.$octet"
}

if [ ! -f "$CONFIG_FILE" ]; then
    echo "Config file not found: $CONFIG_FILE" >&2
    exit 1
fi

COORDINATOR_ADDR=$(sed -nE 's/^[[:space:]]*coordinator:[[:space:]]*"?([^"]+)"?/\1/p' "$CONFIG_FILE" | head -n1)
SERVER_ADDRESS=$(sed -nE 's/^[[:space:]]*server_address:[[:space:]]*"?([^"]+)"?/\1/p' "$CONFIG_FILE" | head -n1)

COORDINATOR_HOST="${COORDINATOR_ADDR%:*}"

if [ -z "$COORDINATOR_ADDR" ] || [ -z "$SERVER_ADDRESS" ]; then
    echo "Invalid config.yml" >&2
    exit 1
fi

echo "Coordinator: $COORDINATOR_ADDR"
echo "Server address: $SERVER_ADDRESS"

if [ "$COORDINATOR_HOST" = "127.0.0.1" ] || [ "$COORDINATOR_HOST" = "localhost" ]; then
    STATIC_IP=$(generate_ip)
    echo "Bootstrap node. Generated IP: $STATIC_IP"

    sed -i -E "s|^[[:space:]]*coordinator:.*$|coordinator: \"$STATIC_IP:7000\"|" "$CONFIG_FILE"
    sed -i -E "s|^[[:space:]]*server_address:.*$|server_address: \"$STATIC_IP:7000\"|" "$CONFIG_FILE"

    IP_TO_USE="$STATIC_IP"
else
    NEW_IP=$(generate_ip)
    echo "Joining node. Generated IP: $NEW_IP"

    sed -i -E "s|^[[:space:]]*server_address:.*$|server_address: \"$NEW_IP:8000\"|" "$CONFIG_FILE"

    IP_TO_USE="$NEW_IP"
fi

if [ "${SKIP_PODMAN:-0}" = "1" ]; then
    exit 0
fi

if ! "$PODMAN_BIN" network exists "$NETWORK_NAME"; then
    "$PODMAN_BIN" network create --subnet "${SUBNET}.0/24" "$NETWORK_NAME"
fi

CONTAINER_ID=$("$PODMAN_BIN" run -d \
    --name "$CONTAINER_NAME" \
    --network "$NETWORK_NAME" \
    --ip "$IP_TO_USE" \
    --volume "$(pwd)/etc:/project/etc:Z" \
    "$IMAGE_NAME")

echo "Container '$CONTAINER_NAME' ($CONTAINER_ID) started with IP $IP_TO_USE"

"$PODMAN_BIN" logs -f "$CONTAINER_ID"
