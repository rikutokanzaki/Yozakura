#!/bin/bash

# Uninstallation process begins
if sudo -v; then
  echo "Authentication succeeded."
else
  echo "Authentication failed."
  exit 1
fi

set -a
source ./.env
set +a

COMPOSE_DIR=./compose

COMPOSE_FILE_STANDARD=${COMPOSE_DIR}/standard.yml
COMPOSE_FILE_SSH=${COMPOSE_DIR}/ssh.yml
COMPOSE_FILE_HTTP=${COMPOSE_DIR}/http.yml

echo
echo "Deleting services with Docker Compose..."
echo

if ! docker compose -f "$COMPOSE_FILE_STANDARD" down --rmi all --volumes --remove-orphans; then
  echo "Error: docker compose down failed."
  exit 1
fi

echo
echo "Handling data backup..."
echo

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_NAME="$(basename "$SCRIPT_DIR")"

INSTALL_DATE_FILE=".install_date"
TODAY=$(date +"%Y%m%d")
TIMENOW=$(date +"%H%M%S")

if [ -f "$INSTALL_DATE_FILE" ]; then
  INSTALL_DATE=$(cat "$INSTALL_DATE_FILE")
else
  INSTALL_DATE="unknown"
fi

PERIOD_DIR="${INSTALL_DATE}-${TODAY}-${TIMENOW}"

ARCHIVE_BASE="../archive/${PROJECT_NAME}"
TARGET_DIR="${ARCHIVE_BASE}/${PERIOD_DIR}"

if [ -d "./data" ]; then
  mkdir -p "$TARGET_DIR"
  echo "Copying ./data → ${TARGET_DIR}/data"
  cp -a ./data "${TARGET_DIR}/data"
  echo "Data copied successfully."
else
  echo "No ./data directory found. Skipping."
fi

echo
echo "Uninstallation complete."
