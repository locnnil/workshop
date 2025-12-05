# sdkcraft clean

## Usage:

```text
sdkcraft clean [options] <parts>
```

## Summary:

Clean up artifacts belonging to parts. If no parts are specified, remove the packing environment.

## Positional arguments:

| Argument | Description |
|-|-|
| `parts` | Optional list of parts to process |

## Options:

| Option | Description |
|-|-|
| `--destructive-mode` | Build in the current host |
| `--use-lxd` | Build in a LXD container. |
| `--platform` | Platform to clean |

## See also:

- `build`
- `pack`
- `prime`
- `pull`
- `stage`
- `try`

