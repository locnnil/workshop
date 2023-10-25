#!/bin/bash

# Install and uninstall workshopd

function install_workshopd() {
  # make sure there is no existing changes
    go install -buildvcs=false /remote/cmd/workshopd
    install /remote/cmd/workshopd/workshopd.service /etc/systemd/system/
    mkdir -p /etc/systemd/system/workshopd.service.d
    echo "[Service]
Environment=\"SDK_STORE_URL=http://localhost:8080/storage/v1/\"
Environment=\"WORKSHOP_DEBUG=1\"" > /etc/systemd/system/workshopd.service.d/local.conf
    systemctl daemon-reload
    systemctl start workshopd
}

function uninstall_workshopd() {
  systemctl stop workshopd
  rm -f /etc/systemd/system/workshopd.service
  rm -f "$HOME"/go/bin/workshopd
  rm -rf "$WORKSHOP"
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

# Functions to assert required LXD state

function assert_workshop_config() {
  project_id=$(cat "$1"/.workshop.lock)
  lxc config device get ws-"$project_id" workshop.project source --project workshop.ubuntu | MATCH "$1"
  lxc config device get ws-"$project_id" workshop.project path --project workshop.ubuntu | MATCH /project
  lxc config get ws-"$project_id" user.workshop.project-id --project workshop.ubuntu | MATCH "$project_id"

  lxc list -c ns -f compact --project workshop.ubuntu | MATCH "ws-$project_id  RUNNING"
}

function assert_workshop_sdk() {
  project_id=$(cat "$1"/.workshop.lock)
  workshop=ws-"$project_id"
  base=/var/lib/workshop/sdk
  sdk_config=$(lxc config get "$workshop" user.workshop.sdk --project workshop.ubuntu)

  rev=$(lxc exec "$workshop" -- ls -1 "$base"/"$2" | sort | grep -E "^[0-9]+$")
  echo "$sdk_config" | jq ".\"$2\"[0].revision" | MATCH "$rev"
  echo "$sdk_config" | jq ".\"$2\"[0].channel" | MATCH "latest/stable"

  lxc exec "$workshop" -- test -h "$base"/"$2"/current || echo "current must be a symbolic link"
  lxc exec "$workshop" -- test "$base"/"$2"/"$rev" = "$(readlink -f "$base"/"$2"/current)" || echo "current does not point to $rev"
}

# General functions

function cleanup() {
  lxc delete $(lxc list -c n -f csv --project workshop.ubuntu) --force --project workshop.ubuntu
  lxc project set workshop.ubuntu user.workshop.projects ""
  for i in "$1"/*; do
    rm -f "$i"/.workshop.lock
  done
}

# Workshop sub-command wrappers

function workshop_exec() {
  sudo -u ubuntu 2>&1 -- workshop "$@"
}

function launch() {
  sudo -u ubuntu -- workshop --project "$1" launch "$2"
}

function list() {
    sudo -u ubuntu -- workshop --project "$1" list
}

function list_cwd() {
  sudo -u ubuntu -- workshop list
}

function list_global() {
  sudo -u ubuntu -- workshop list --global
}

function delete() {
    lxc delete "$1" --force --project workshop.ubuntu
}

function changes() {
  sudo -u ubuntu -- workshop changes --project "$1"
}

function changes_global() {
  sudo -u ubuntu -- workshop changes
}
