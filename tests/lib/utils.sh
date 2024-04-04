#!/bin/bash

# Install and uninstall workshopd

function prepare_environment() {
  snap install go --classic

  snap install lxd --classic
  lxd init --auto --storage-backend zfs
  
  snap install yq
  apt install jq -y --no-install-recommends
  
  snap install --dangerous --classic /workshop/tests/*.snap
  snap set workshop store.url=http://localhost:8080/storage/v1/
  snap restart workshop
}

function cleanup_environment() {
  snap remove workshop --purge
  snap remove lxd
}

function start_sdk_store() {
    # run fake GCS bucket storage to emulate SDK store
  publish_test_sdk_content "$SDKCONTENT" "$SDK_STORE_BUCKET_DIR"
  chown -R ubuntu.ubuntu /data # a bug with fake-gcs-server that returns 404 if not owned by the user
  mkdir -p /storage
  chown -R ubuntu.ubuntu /storage
  
  /bin/sh -c "nohup go run github.com/fsouza/fake-gcs-server@latest -data /data -scheme http -port 8080 -public-host localhost:8080 > ~/fake_sdk_store.log 2>&1 &"

  echo "Waiting for the fake SDK store to start on port 8080..."
  while ! nc -z localhost 8080; do
    sleep 0.1
  done
}

function stop_sdk_store() {
  pkill -f fake-gcs-server || true
  rm -rf /data
  rm -rf /storage
}

# Publish test SDKs in the fake SDK Store
function publish_test_sdk_content() {
    for i in "$1"/*; do
      SDK_NAME=$(basename "$i")
      SDK_FILE=$SDK_NAME.sdk
      SDK_PATH=$(readlink -f "$i")
      STORE_PATH="$2"/"$SDK_NAME"/latest/stable/

      tar czf "$SDK_FILE" -C "$SDK_PATH" $(ls -A "$SDK_PATH")
      mkdir -p "$STORE_PATH"
      mv "$SDK_FILE" "$STORE_PATH"
    done
}

# General functions
function cleanup() {
  lxc delete $(lxc list -c n -f csv --project workshop.ubuntu) --force --project workshop.ubuntu
  lxc project set workshop.ubuntu user.workshop.projects ""
  for i in "$1"/*/; do
    rm -f "$i"/.workshop.lock
  done
}

# Workshop sub-command wrappers
function workshop_exec() {
  sudo -u ubuntu 2>&1 -- workshop "$@"
}
