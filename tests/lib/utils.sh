#!/bin/bash

function assert_workspace_config() {
  lxc config device get ws-$2 workspace.project source --project workspace.ubuntu | MATCH $1
  lxc config device get ws-$2 workspace.project path --project workspace.ubuntu | MATCH /project
  lxc config get ws-$2 user.workspace.project-id --project workspace.ubuntu | MATCH $2

  lxc list -c ns -f compact --project workspace.ubuntu | MATCH "ws-$2  RUNNING"
}

function cleanup_workspaces_in_tree() {
  lxc delete `lxc list -c n -f csv --project workspace.ubuntu` --force --project workspace.ubuntu
  for i in $1/*; do
    rm -f $i/.workspace.lock
  done
}

function launch() {
  sudo -u ubuntu -- workspace --project $1 launch $2
}

function list() {
    sudo -u ubuntu -- workspace --project $1 list
}

function list_all() {
    sudo -u ubuntu -- workspace list --all
}

function delete() {
    lxc delete $1 --force --project workspace.ubuntu
}
  