# sdkcraft prime

## Usage:

```text
sdkcraft prime [options] <parts>
```

## Summary:

Prepare the final payload to be packed, performing additional processing and adding metadata files. If part names are specified only those parts will be primed. The default is to prime all parts.

## Positional arguments:

| Argument | Description |
|-|-|
| `parts` | Optional list of parts to process |

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

## See also:

- `build`
- `clean`
- `pack`
- `pull`
- `stage`
- `try`

