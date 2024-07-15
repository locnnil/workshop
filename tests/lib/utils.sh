#!/bin/bash

# Install and uninstall workshopd

function init_lxd() {
  snap install --classic lxd
  snap refresh --channel=5.21/stable lxd
  
  # can already be initialised if reused
  # https://discuss.linuxcontainers.org/t/how-do-i-know-if-lxd-is-initialized/15473/3
  if [ $(lxc storage list -f compact | grep -c default) -eq 0  ]; then   
      lxd init --auto --storage-backend=zfs
  fi
}

function prepare_environment() {
  systemctl unmask snapd.service
  systemctl start snapd.service
  
  init_lxd  
  snap install --classic --channel=1.21/stable go
  snap install yq
  apt update
  apt install -y --no-install-recommends moreutils jq
  
  snap install --dangerous --classic /workshop/tests/*.snap
  snap set workshop store.url=http://localhost:8080/storage/v1/
  snap restart workshop
}

function cleanup_environment() {
  snap remove workshop --purge
  lxc delete $(lxc list -c n -f csv --project workshop.ubuntu) --force --project workshop.ubuntu
  lxc project set workshop.ubuntu user.workshop.projects ""
  find /workshop -name .workshop.lock -delete
}

function start_sdk_store() {
    # run fake GCS bucket storage to emulate SDK store
  publish_test_sdk_content "$SDKS" "$SDK_STORE_BUCKET_DIR"
  chown -R ubuntu.ubuntu /data # a bug with fake-gcs-server that returns 404 if not owned by the user
  mkdir -p /storage
  chown -R ubuntu.ubuntu /storage
  
  /bin/sh -c "nohup go run github.com/fsouza/fake-gcs-server@latest -data /data -scheme http -port 8080 -public-host localhost:8080 > ~/fake_sdk_store.log 2>&1 &"

  echo "Waiting for the fake SDK store to start on port 8080..."
  while ! nc -z localhost 8080; do
    sleep 2
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
      cp -f "$SDK_PATH"/meta/sdk.yaml "$STORE_PATH"
    done
}

# Workshop sub-command wrappers
function workshop_exec() {
  sudo -u ubuntu 2>&1 -- workshop "$@"
}
