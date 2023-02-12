# Workspace

## Installation

### Prerequisites

Workspace relies on LXD to orchestrate contatainers. To install and configure LXD:

```sh
sudo snap install lxd
sudo lxd init --auto
```

### Install

```
go install github.com/canonical/workspace/cmd/workspace
```

## Use

```
$ cat > .workspace.finbert.yaml <<EOF -
name: finbert
base: ubuntu@20.04
sdks:
  huggingface:
    channel: latest/stable
  cuda:
    channel: latest/stable
EOF
$ workspace launch
```

## Testing

### Unit tests

Mocks boilerplate can be updated or created with mockery. Use _--dry-run_ if in doubt.
```
go install github.com/vektra/mockery/v2@v2.20.0
mockery --name=WorkspaceServer --structname=MockWorkspaceServer --dir=./internal/server/ --output=./internal/mocks/
```

### Functional and integrational testing

See [QEMU backend](https://github.com/snapcore/spread#qemu-backend) for prerequisites.

```
go install github.com/snapcore/spread/cmd/spread@latest
spread 
```



## TODO

- Validate bases used for the workspace and SDKs (must be the same)
- Avoid potential conflicts for SDK blobs if used concurrently
- Create a separate network for the workspace's instances
- Tests for the workspace YAML validation
- Use logger and logging levels instead of fmt