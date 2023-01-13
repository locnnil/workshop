# Workspace

## Installation

### Prerequisites

Workspace relies on LXD to orchestrate contatainers. To install and configure LXD:

```sh
sudo snap install lxd
sudo lxd init --minimal
```

### Install

```
go install ./cmd/workspace
```

## Use

```
$ cat > .workspace.project.yaml <<EOF -
name: test
base: ubuntu@20.04
sdks:
  huggingface:
    channel: latest/stable
  cuda:
    channel: latest/stable
EOF
$ workspace launch
```

## TODO

- Validate workspace name and base names
- Validate bases used for the workspace and SDKs (must be the same)
- Avoid potential conflicts for SDK blobs if used concurrently
- Ensure only the latest/stable channel can be used
