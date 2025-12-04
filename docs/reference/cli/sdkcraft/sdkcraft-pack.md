# sdkcraft pack

## Usage:

```text
sdkcraft pack [options]
```

## Summary:

Process parts and create the final artifact.

## Options:

| Option | Description |
|-|-|
| `--destructive-mode` | Build in the current host |
| `--use-lxd` | Build in a LXD container. |
| `--shell` | Shell into the environment in lieu of the step to run. |
| `--shell-after` | Shell into the environment after the step has run. |
| `--debug` | Shell into the environment if the build fails. |
| `--platform` | Set platform to build for |
| `--build-for` | Set architecture to build for |
| `--output, -o` | Output directory for created packages. |

## See also:

- `build`
- `clean`
- `prime`
- `pull`
- `stage`
- `try`

