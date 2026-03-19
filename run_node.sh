#!/bin/bash

SUBNET="192.168.100"    # Subnet /24
PORT="8080"             
CONFIG_FILE="etc/config.yml"
CONTAINER_NAME="fixed-partitioning"
IMAGE_NAME="localhost/project"

generate_ip() {
    local octet=$((RANDOM % 254 + 1))   
    echo "$SUBNET.$octet"
}

IP=$(generate_ip)
echo "generated IP: $IP"

if [ -f "$CONFIG_FILE" ]; then
    sed -i "s|^\s*server_address:.*|server_address: $IP:$PORT|" "$CONFIG_FILE"
    echo "update config $CONFIG_FILE"
else
    echo "Error: $CONFIG_FILE not found!"
    exit 1
fi

podman network create --subnet "${SUBNET}.0/24" mynet 2>/dev/null
podman run -d --name "$CONTAINER_NAME" --network mynet --ip "$IP" "$IMAGE_NAME"

echo "Container '$CONTAINER_NAME' running on  $IP:$PORT"
