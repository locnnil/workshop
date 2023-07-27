#!/bin/bash

# Install and uninstall workspaced

function install_workspaced() {
  # make sure there is no existing changes
    go install -buildvcs=false /remote/cmd/workspaced
    install /remote/cmd/workspaced/workspaced.service /etc/systemd/system/
    systemctl start workspaced
}

function uninstall_workspaced() {
  systemctl stop workspaced
  rm -f /etc/systemd/system/workspaced.service
  rm -f "$HOME"/go/bin/workspaced
  rm -rf "$WORKSPACE"
}
# Functions to assert required LXD state

function assert_workspace_config() {
  project_id=$(cat "$1"/.workspace.lock)
  lxc config device get ws-"$project_id" workspace.project source --project workspace.ubuntu | MATCH "$1"
  lxc config device get ws-"$project_id" workspace.project path --project workspace.ubuntu | MATCH /project
  lxc config get ws-"$project_id" user.workspace.project-id --project workspace.ubuntu | MATCH "$project_id"

  lxc list -c ns -f compact --project workspace.ubuntu | MATCH "ws-$project_id  RUNNING"
}

function assert_workspace_sdk() {
  project_id=$(cat "$1"/.workspace.lock)
  workspace=ws-"$project_id"
  base=/var/lib/workspace/sdk
  sdk_config=$(lxc config get "$workspace" user.workspace.sdk --project workspace.ubuntu)

  rev=$(lxc exec "$workspace" -- ls -1 "$base"/"$2" | sort | grep -E "^[0-9]+$")
  echo "$sdk_config" | jq ".\"$2\"[0].revision" | MATCH "$rev"
  echo "$sdk_config" | jq ".\"$2\"[0].channel" | MATCH "latest/stable"

  lxc exec "$workspace" -- test -h "$base"/"$2"/current || echo "current must be a symbolic link"
  lxc exec "$workspace" -- test "$base"/"$2"/"$rev" = "$(readlink -f "$base"/"$2"/current)" || echo "current does not point to $rev"
}

# General functions

function cleanup() {
  lxc delete $(lxc list -c n -f csv --project workspace.ubuntu) --force --project workspace.ubuntu
  lxc project set workspace.ubuntu user.workspace.projects ""
  for i in $1/*; do
    rm -f "$i"/.workspace.lock
  done
}

# Workspace sub-command wrappers

function launch() {
  sudo -u ubuntu -- workspace --project "$1" launch "$2"
}

function refresh() {
  sudo -u ubuntu -- workspace --project "$1" refresh "$2"
}

function list() {
    sudo -u ubuntu -- workspace --project "$1" list
}

function list_cwd() {
  sudo -u ubuntu -- workspace list
}

function list_global() {
  sudo -u ubuntu -- workspace list --global
}

function delete() {
    lxc delete "$1" --force --project workspace.ubuntu
}

function changes() {
  sudo -u ubuntu -- workspace changes --project "$1"
}

function changes_global() {
  sudo -u ubuntu -- workspace changes
}
