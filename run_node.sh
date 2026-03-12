#!/usr/bin/env bash
set -euo pipefail

# ------------------------------------------------------------------------------
# Podman container installer
#
# Features:
# - Ensures a Podman network exists with the required subnet
# - Selects an available static IP inside the subnet (10..250)
# - Updates server_address (or server.ip) in YAML (prefers yq, falls back to sed)
# - Starts the container on the network using the selected static IP
# - Handles "container already exists" safely
# ------------------------------------------------------------------------------

NETWORK_NAME="my-network"
SUBNET="10.10.0.0/24"
CONTAINER_NAME="my-container"

# Path to the YAML config to update. Override with: CONFIG_FILE=/path/to/config.yml
CONFIG_FILE="${CONFIG_FILE:-}"

detect_config_file() {
  # Auto-detect a reasonable default if CONFIG_FILE isn't explicitly set.
  local candidates=(
    "etc/config.yml"
    "etc/config.yaml"
    "config.yml"
    "config.yaml"
  )

  local c
  for c in "${candidates[@]}"; do
    if [[ -f "$c" ]]; then
      printf '%s\n' "$c"
      return 0
    fi
  done

  # Default fallback:
  # - If there's an ./etc directory, assume the config belongs there (prefer .yml).
  # - Otherwise fall back to ./config.yml.
  if [[ -d "etc" ]]; then
    printf '%s\n' "etc/config.yml"
  else
    printf '%s\n' "config.yml"
  fi
}

# If set to 1, creates a minimal config file when missing (safe default is to fail).
CREATE_CONFIG_IF_MISSING="${CREATE_CONFIG_IF_MISSING:-0}"

# Image must be provided via environment variable IMAGE (or inline: IMAGE=... ./run_node.sh)
IMAGE="${IMAGE:-}"

log() { printf '[%s] %s\n' "$(date +'%Y-%m-%d %H:%M:%S')" "$*" >&2; }
die() { printf '[%s] ERROR: %s\n' "$(date +'%Y-%m-%d %H:%M:%S')" "$*" >&2; exit 1; }

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "Missing required command: $1"
}

podman_container_exists() {
  # podman has "container exists" in newer versions; fall back if unavailable.
  if podman container exists "$1" >/dev/null 2>&1; then
    return 0
  fi
  podman ps -a --format '{{.Names}}' 2>/dev/null | awk -v n="$1" '$0==n{found=1} END{exit(found?0:1)}'
}

podman_container_running() {
  local name="$1"
  local status
  status="$(podman inspect -f '{{.State.Status}}' "$name" 2>/dev/null || true)"
  [[ "$status" == "running" ]]
}

ensure_network() {
  log "Checking Podman network '$NETWORK_NAME'..."
  if podman network inspect "$NETWORK_NAME" >/dev/null 2>&1; then
    log "Network exists."

    # Best-effort subnet verification; if we can read subnets and it doesn't match, abort.
    local subnets
    subnets="$(podman network inspect "$NETWORK_NAME" --format '{{range .Subnets}}{{.Subnet}}{{"\n"}}{{end}}' 2>/dev/null || true)"
    if [[ -n "$subnets" ]] && ! grep -qxF "$SUBNET" <<<"$subnets"; then
      die "Network '$NETWORK_NAME' exists but subnet does not include '$SUBNET'. Found: $(tr '\n' ' ' <<<"$subnets" | sed -E 's/[[:space:]]+/ /g')"
    fi
    return 0
  fi

  log "Network not found. Creating '$NETWORK_NAME' with subnet '$SUBNET'..."
  podman network create --subnet "$SUBNET" "$NETWORK_NAME" >/dev/null
  log "Network created."
}

network_base_ip() {
  # Only supports /24 as requested (10.10.0.0/24 -> 10.10.0)
  local cidr="${SUBNET%/*}"      # 10.10.0.0
  local mask="${SUBNET#*/}"      # 24
  [[ "$mask" == "24" ]] || die "Only /24 subnets are supported by this script (got /$mask)."
  local base="${cidr%.*}"        # 10.10.0
  local last="${cidr##*.}"       # 0
  [[ "$last" == "0" ]] || die "Expected a /24 base address ending in .0 (got $cidr)."
  printf '%s' "$base"
}

list_used_ips_on_network() {
  # Gathers IPv4 addresses currently assigned to containers attached to the network.
  # Tries fast filter first; falls back to scanning all containers if filter isn't supported.
  local ids=""
  if podman ps -a --filter "network=$NETWORK_NAME" --format '{{.ID}}' >/dev/null 2>&1; then
    ids="$(podman ps -a --filter "network=$NETWORK_NAME" --format '{{.ID}}' 2>/dev/null || true)"
  else
    ids="$(podman ps -a --format '{{.ID}}' 2>/dev/null || true)"
  fi

  local id ip
  while read -r id; do
    [[ -n "$id" ]] || continue
    ip="$(podman inspect -f '{{with (index .NetworkSettings.Networks "'"$NETWORK_NAME"'")}}{{.IPAddress}}{{end}}' "$id" 2>/dev/null || true)"
    [[ -n "$ip" ]] && printf '%s\n' "$ip"
  done <<<"$ids" | awk 'NF' | sort -u
}

generate_available_ip() {
  local base; base="$(network_base_ip)"

  declare -A used=()
  local ip
  while read -r ip; do
    [[ -n "$ip" ]] && used["$ip"]=1
  done < <(list_used_ips_on_network)

  log "Selecting an available static IP in ${SUBNET} (avoiding .0, .1, broadcast; using .10-.250)..."

  # Randomized starting point (no external deps); then wrap around.
  local start offset octet candidate
  start=$(( (RANDOM % 241) + 10 ))  # 10..250
  for ((offset=0; offset<=240; offset++)); do
    octet=$(( start + offset ))
    if (( octet > 250 )); then
      octet=$(( octet - 241 ))
    fi
    candidate="${base}.${octet}"

    if [[ -z "${used[$candidate]+x}" ]]; then
      printf '%s\n' "$candidate"
      return 0
    fi
  done

  die "No available IPs found in ${base}.10-${base}.250 (all appear to be in use)."
}

update_config_yaml() {
  local ip="$1"
  if [[ -z "${CONFIG_FILE}" ]]; then
    CONFIG_FILE="$(detect_config_file)"
  fi
  if [[ ! -f "$CONFIG_FILE" ]]; then
    if [[ "$CREATE_CONFIG_IF_MISSING" == "1" ]]; then
      log "Config file '$CONFIG_FILE' not found; creating a minimal one..."
      cat >"$CONFIG_FILE" <<'YAML'
server_address: "0.0.0.0:7000"
YAML
    else
      die "Config file not found: $CONFIG_FILE (run from the directory containing it, set CONFIG_FILE=..., or set CREATE_CONFIG_IF_MISSING=1)"
    fi
  fi

  log "Updating '$CONFIG_FILE' with generated IP ($ip)..."

  # Prefer mikefarah yq if installed and usable.
  if command -v yq >/dev/null 2>&1; then
    if yq --version 2>/dev/null | grep -qi 'mikefarah'; then
      # Prefer updating server_address if present (e.g. "127.0.0.1:7000"), otherwise update server.ip.
      if yq -e 'has("server_address")' "$CONFIG_FILE" >/dev/null 2>&1; then
        local old_addr port
        old_addr="$(yq -r '.server_address // ""' "$CONFIG_FILE" 2>/dev/null || true)"
        if [[ "$old_addr" =~ :([0-9]+)$ ]]; then
          port="${BASH_REMATCH[1]}"
        else
          die "Could not parse port from server_address in $CONFIG_FILE (value: $old_addr)"
        fi
        NEW_SERVER_ADDRESS="${ip}:${port}" yq -i '.server_address = env(NEW_SERVER_ADDRESS)' "$CONFIG_FILE" >/dev/null 2>&1
        log "Updated server_address via yq."
        return 0
      fi

      if yq -e '.server.ip' "$CONFIG_FILE" >/dev/null 2>&1; then
        GENERATED_IP="$ip" yq -i '.server.ip = env(GENERATED_IP)' "$CONFIG_FILE" >/dev/null 2>&1
        log "Updated server.ip via yq."
        return 0
      fi

      die "No supported key found to update in $CONFIG_FILE (expected server_address or server.ip)."
    else
      log "yq detected (non-mikefarah variant); falling back to sed for safety."
    fi
  fi

  # sed fallback: prefer updating server_address: "IP:PORT" while preserving PORT.
  if grep -qE '^[[:space:]]*server_address:[[:space:]]*' "$CONFIG_FILE"; then
    local port
    port="$(awk '
      /^[[:space:]]*server_address:[[:space:]]*/ {
        # Match: server_address: "127.0.0.1:7000"
        if (match($0, /server_address:[[:space:]]*\"[^\":]+:([0-9]+)\"/, a)) { print a[1]; exit 0 }
        # Match: server_address: 127.0.0.1:7000
        if (match($0, /server_address:[[:space:]]*[^[:space:]]+:([0-9]+)$/, a)) { print a[1]; exit 0 }
        exit 1
      }
    ' "$CONFIG_FILE" 2>/dev/null || true)"
    [[ -n "$port" ]] || die "Could not parse port from server_address in $CONFIG_FILE (install yq or fix the value format)"

    # Replace only the IP part, preserve the existing port. Force a quoted value on output.
    sed -i -E "s|^([[:space:]]*server_address:[[:space:]]*)\"?[0-9.]+:${port}\"?([[:space:]]*)$|\\1\"${ip}:${port}\"\\2|" "$CONFIG_FILE"
    log "Updated server_address via sed."
    return 0
  fi

  # Otherwise, fall back to updating server.ip under a top-level server: block.
  if ! grep -qE '^server:[[:space:]]*$' "$CONFIG_FILE"; then
    die "No supported key found to update in $CONFIG_FILE (expected server_address or a top-level server: block)."
  fi

  if awk '
    BEGIN{in_server=0; found=0}
    /^server:[[:space:]]*$/ {in_server=1; next}
    in_server && /^[^[:space:]]/ {in_server=0}
    in_server && /^[[:space:]]*ip:[[:space:]]*/ {found=1}
    END{exit(found?0:1)}
  ' "$CONFIG_FILE"; then
    sed -i -E "/^server:[[:space:]]*$/,/^[^[:space:]]/{s/^([[:space:]]*)ip:[[:space:]]*.*/\1ip: ${ip}/;}" "$CONFIG_FILE"
  else
    die "Found top-level 'server:' but no 'ip:' key to update in $CONFIG_FILE (install yq or add server.ip)"
  fi

  log "Updated via sed."
}

start_container() {
  local ip="$1"

  [[ -n "$IMAGE" ]] || die "IMAGE is not set. Example: IMAGE='docker.io/library/nginx:latest' $0"

  if podman_container_exists "$CONTAINER_NAME"; then
    if podman_container_running "$CONTAINER_NAME"; then
      log "Container '$CONTAINER_NAME' already exists and is running; leaving it as-is."
      log "If you want to redeploy, stop/remove it first: podman stop '$CONTAINER_NAME' && podman rm '$CONTAINER_NAME'"
      exit 0
    fi
    log "Container '$CONTAINER_NAME' already exists but is not running; removing it..."
    podman rm "$CONTAINER_NAME" >/dev/null
  fi

  log "Starting container '$CONTAINER_NAME' from image '$IMAGE' on network '$NETWORK_NAME' with IP '$ip'..."
  podman run -d \
    --name "$CONTAINER_NAME" \
    --network "$NETWORK_NAME" \
    --ip "$ip" \
    "$IMAGE" >/dev/null

  log "Container started."
}

main() {
  need_cmd podman

  ensure_network

  # Handle container existence early to free its old IP (if it was stopped).
  if podman_container_exists "$CONTAINER_NAME" && ! podman_container_running "$CONTAINER_NAME"; then
    log "Found existing stopped container '$CONTAINER_NAME'; removing to free resources/IP..."
    podman rm "$CONTAINER_NAME" >/dev/null
  fi

  local generated_ip
  generated_ip="$(generate_available_ip)"
  if [[ ! "$generated_ip" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]]; then
    die "Generated IP is invalid (got: $generated_ip)"
  fi
  log "Selected IP: $generated_ip"

  update_config_yaml "$generated_ip"
  start_container "$generated_ip"

  log "Done."
}

main "$@"
