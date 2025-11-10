# sdkcraft try

## Usage:

```text
sdkcraft try [options] <sdks>
```

## Summary:

Pack the SDK and copy it to the Workshop try area.

## Positional arguments:

| | |
|-|-|
| `sdks` | Skip packing and try out specific SDK files. |

## Options:

| | |
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

