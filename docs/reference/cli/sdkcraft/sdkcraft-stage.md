# sdkcraft stage

## Usage:

```text
sdkcraft stage [options] <parts>
```

## Summary:

Stage built artifacts into a common staging area. If part names are specified only those parts will be staged. The default is to stage all parts.

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

- `build`
- `clean`
- `pack`
- `prime`
- `pull`
- `try`

