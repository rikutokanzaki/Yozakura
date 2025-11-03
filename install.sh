#!/bin/bash

YOZAKURAINSTALLER=$(
  cat <<"EOF"
__   __            _                       ___           _        _ _
\ \ / /__ ______ _| | ___   _ _ __ __ _   |_ _|_ __  ___| |_ __ _| | | ___ _ __
 \ V / _ \_  / _` | |/ / | | | '__/ _` |   | || '_ \/ __| __/ _` | | |/ _ \ '__|
  | | (_) / / (_| |   <| |_| | | | (_| |   | || | | \__ \ || (_| | | |  __/ |
  |_|\___/___\__,_|_|\_\\__,_|_|  \__,_|  |___|_| |_|___/\__\__,_|_|_|\___|_|
EOF
)
# Installation process begins
if sudo -v; then
  echo "Authentication succeeded."
else
  echo "Authentication failed."
  exit 1
fi

set -a
source ./.env
set +a

if [ -z "$KIBANA_PASSWORD" ]; then
  echo "Error: KIBANA_PASSWORD is not set."
  exit 1
fi

COMPOSE_DIR=./compose

mapfile -t COMPOSE_FILES < <(find "$COMPOSE_DIR" -maxdepth 1 -type f \( -name "*.yml" -o -name "*.yaml" \) | sort)

if [ ${#COMPOSE_FILES[@]} -eq 0 ]; then
  echo "No compose files found in $COMPOSE_DIR"
  exit 1
fi

echo "Select honeypot to launch (Enter=1)"

for i in "${!COMPOSE_FILES[@]}"; do
  fname=$(basename "${COMPOSE_FILES[$i]}")
  name="${fname%.*}"
  printf "  %2d) %-12s -> %s\n" "$((i+1))" "$name" "${COMPOSE_FILES[$i]}"
done

read -p "Selection (number or name): " INPUT
[ -z "$INPUT" ] && INPUT=1

if [[ "$INPUT" =~ ^[0-9]+$ ]]; then
  idx=$((INPUT-1))

  if [ $idx -lt 0 ] || [ $idx -ge ${#COMPOSE_FILES[@]} ]; then
    echo "Error: invalid number: $INPUT"
    exit 1
  fi

  SELECTED_COMPOSE_FILE="${COMPOSE_FILES[$idx]}"

else
  MATCHED=""

  for f in "${COMPOSE_FILES[@]}"; do
    n=$(basename "$f"); n="${n%.*}"

    if [ "$n" = "$INPUT" ]; then
      MATCHED="$f"; break
    fi
  done

  if [ -z "$MATCHED" ]; then
    echo "Error: compose for '$INPUT' not found under $COMPOSE_DIR"
    exit 1
  fi

  SELECTED_COMPOSE_FILE="$MATCHED"
fi

SELECTED_TYPE=$(basename "$SELECTED_COMPOSE_FILE"); SELECTED_TYPE="${SELECTED_TYPE%.*}"

if [ ! -f "$SELECTED_COMPOSE_FILE" ]; then
  echo "Compose file not found: $SELECTED_COMPOSE_FILE"
  exit 1
fi

echo "Selected type: $SELECTED_TYPE"
echo "Selected compose file: $SELECTED_COMPOSE_FILE"
echo

echo "$YOZAKURAINSTALLER"
echo
echo

sudo mkdir -p -m 777 ./data/cowrie
sudo chown root:root ./data/cowrie
sudo mkdir -p -m 755 ./data/wordpot/log
sudo chown 2000:2000 ./data/wordpot/log
sudo mkdir -p -m 755 ./data/h0neytr4p/log
sudo chown 2000:2000 ./data/h0neytr4p/log
sudo mkdir -p -m 755 ./data/h0neytr4p/payloads
sudo chown 2000:2000 ./data/h0neytr4p/payloads
sudo mkdir -p -m 755 ./data/heralding
sudo chown 2000:2000 ./data/heralding
sudo chmod 444 ./elk/metricbeat/metricbeat.yml
sudo chown root:root ./elk/metricbeat/metricbeat.yml


echo "Starting services with Docker Compose..."
if ! docker compose -f "$SELECTED_COMPOSE_FILE" up -d; then
  echo "Error: docker compose up failed. Aborting without stopping services or importing Kibana objects."
  exit 1
fi

echo
echo "Importing Kibana saved objects..."
echo

response=$(curl -s -w "\n%{http_code}" -X POST http://127.0.0.1:64297/api/saved_objects/_import?createNewCopies=true \
  -u elastic:"$KIBANA_PASSWORD" \
  -H "kbn-xsrf: true" \
  -F file=@./elk/kibana/export.ndjson)

body=$(echo "$response" | sed '$d')
status=$(echo "$response" | tail -n1)

echo
echo "$body"
echo

if [ "$status" = "200" ]; then
  echo "Kibana saved objects completely imported."
else
  echo "Failed to import Kibana saved objects. HTTP status: $status"
fi

date +"%Y%m%d" > .install_date

echo
echo "Installation date recorded."
