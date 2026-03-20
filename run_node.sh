#!/bin/bash

SUBNET="192.168.100"        # /24 subnet
COORDINATOR_PORT="7000"
SERVER_PORT="8080"
CONFIG_FILE="etc/config.yml"
CONTAINER_NAME="fixed-partitioning-$(date +%s)"
IMAGE_NAME="localhost/project"
NETWORK_NAME="mynet"

generate_ip() {
    local octet=$((RANDOM % 254 + 1))   # avoid .0 and .255
    echo "$SUBNET.$octet"
}

COORDINATOR=$(grep 'coordinator:' "$CONFIG_FILE" | awk -F'"' '{print $2}' | cut -d':' -f1)
SERVER_ADDRESS=$(grep 'server_address:' "$CONFIG_FILE" | awk -F':' '{print $1}')

echo "Current coordinator: $COORDINATOR"
echo "Current server_address: $SERVER_ADDRESS"

if [ "$COORDINATOR" == "127.0.0.1" ]; then
    STATIC_IP=$(generate_ip)
    echo "First-time setup, chosen static IP: $STATIC_IP"

    sed -i "s|^\s*coordinator:.*|coordinator: \"$STATIC_IP:$COORDINATOR_PORT\"|" "$CONFIG_FILE"
    sed -i "s|^\s*server_address:.*|server_address: $STATIC_IP:$COORDINATOR_PORT|" "$CONFIG_FILE"
    IP_TO_USE="$STATIC_IP"
else
    IP_TO_USE=$(generate_ip)
    echo "Updating server_address only: $IP_TO_USE"
    sed -i "s|^\s*server_address:.*|server_address: $IP_TO_USE:$SERVER_PORT|" "$CONFIG_FILE"
fi

if ! podman network exists "$NETWORK_NAME"; then
    podman network create --subnet "${SUBNET}.0/24" "$NETWORK_NAME"
fi

podman run -d --name "$CONTAINER_NAME" --network "$NETWORK_NAME" --ip "$IP_TO_USE" "$IMAGE_NAME"

echo "Container '$CONTAINER_NAME' started with IP $IP_TO_USE"
