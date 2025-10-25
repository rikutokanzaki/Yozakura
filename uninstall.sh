# !/bin/bash

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
