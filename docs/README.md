# Workspace

**Workspace automates the configuring and management of reproducible development
environments**.

**Use a straightforward YAML to define your development environment**. Workspace
will create a system container, install specified SDKs and packages, and control
its behaviour with life cycle hooks. VS Code, Jupyter Lab and other IDEs can
discover and use your workspace as a work environment. Dispose the environment
when done and keep the host system clean.

**Make the knowledge of your project's dev environments explicit and shared**.
New contributors can start with a single command that launches the required
workspace. It is easier to debug issues in any of the project's supported
environments, perform code reviews or experiment in a separate light-weight
container.

It is common to have a non-trivial project setup with dependencies on particular
Linux distributions, SDKs from multiple publishers, and system and language
packages. Most such projects can organise setup complexity with Workspace.
Examples include AI/ML, Robotics, IoT, EdTech and similar domains.

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

### Functional and integration tests

```
go install github.com/snapcore/spread/cmd/spread@latest
spread
```
