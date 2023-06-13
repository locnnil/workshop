# Workspace

## Installation

### Prerequisites

Workspace relies on LXD to orchestrate containers. To install and configure LXD:

```sh
sudo snap install lxd
sudo lxd init --auto
```

### Install

```
go install github.com/canonical/workspace/cmd/workspace
```

## Use

### Running the daemon

To run the Workspace daemon, set the `$WORKSPACE` environment variable and use the `workspaced run` sub-command:

```
$ mkdir ~/workspace
$ export WORKSPACE=~/workspace
$ go run ./cmd/workspaced run
2021-09-15T01:37:23.962Z [workspaced] Started daemon.
...
```

### Using the CLI client

#### Launch a workspace

Create a workspace file in a project directory and launch the workspace:

```
$ cat > .workspace.nimble.yaml <<EOF -
name: nimble
base: ubuntu@22.04
sdks:
  go:
    channel: latest/stable
  openjdk:
    channel: latest/stable
EOF
$ workspace launch nimble
```

#### List available workspaces

From a project directory:

```
$ workspace list
Project           Workspace    State    Notes
~/Work/pebble     pebble       Ready    -
~/Work/workspace  nimble       Ready    -
```

## Testing

### Unit tests

workspace uses a "go test"-compatible [gocheck](https://pkg.go.dev/gopkg.in/check.v1#section-readme)
```
go test ./...
go test -check.f SuiteName
```

### Functional and integrational tests

```
go install github.com/snapcore/spread/cmd/spread@latest
spread
```
