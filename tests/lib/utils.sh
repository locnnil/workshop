#!/bin/bash

function assert_workspace_config() {
  lxc config device get ws-$2 workspace.project source | MATCH $1
  lxc config device get ws-$2 workspace.project path | MATCH /project
  lxc config get ws-$2 user.workspace.project-id | MATCH $2

  lxc list -c ns -f compact | MATCH "ws-$2  RUNNING"
}
  