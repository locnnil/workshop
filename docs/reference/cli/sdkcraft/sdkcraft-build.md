# sdkcraft build

## Usage:

```text
sdkcraft build [options] <parts>
```

## Summary:

Build artifacts defined for a part. If part names are specified only those parts will be built, otherwise all parts will be built.

## Positional arguments:

| | |
|-|-|
| `parts` | Optional list of parts to process |

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

## See also:

- `clean`
- `pack`
- `prime`
- `pull`
- `stage`
- `try`

