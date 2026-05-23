# fx

[![Test](https://github.com/m-mizutani/fx/actions/workflows/test.yml/badge.svg)](https://github.com/m-mizutani/fx/actions/workflows/test.yml)
[![Lint](https://github.com/m-mizutani/fx/actions/workflows/lint.yml/badge.svg)](https://github.com/m-mizutani/fx/actions/workflows/lint.yml)
[![Gosec](https://github.com/m-mizutani/fx/actions/workflows/gosec.yml/badge.svg)](https://github.com/m-mizutani/fx/actions/workflows/gosec.yml)
[![Trivy](https://github.com/m-mizutani/fx/actions/workflows/trivy.yml/badge.svg)](https://github.com/m-mizutani/fx/actions/workflows/trivy.yml)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

Format Exchange CLI — convert between **JSON / YAML / TOML / HCL / Jsonnet**
on the command line. Comments are preserved across formats whenever both
sides can express them.

## Install

```sh
go install github.com/m-mizutani/fx@latest
```

## Usage

```
fx [--src FORMAT] [--dst FORMAT] [--out PATH] [INPUT]
```

- `--src FORMAT` — input format. Auto-detected from extension or content when omitted.
- `--dst FORMAT` — output format. Inferred from `--out`'s extension when omitted.
- `--out PATH` — output file. Defaults to stdout.
- `INPUT` — input file. Defaults to stdin.

### Examples

```sh
# Convert a YAML file to HCL on stdout
fx --src yaml --dst hcl config.yaml

# Auto-detect everything from extensions
fx --out config.toml config.json

# Pipe stdin to stdout with explicit formats
cat data.yaml | fx --src yaml --dst json
```

## Support matrix

| Format | Decode | Encode | Comment preservation |
|--------|--------|--------|----------------------|
| JSON | ✓ | ✓ | — (no comment grammar) |
| YAML | ✓ | ✓ | ✓ |
| TOML | ✓ | ✓ | △ (library limited) |
| HCL | ✓ | ✓ | △ (best-effort) |
| Jsonnet | ✓ | — (decode-only) | — |

When converting to a format that cannot express comments (e.g. JSON),
comments from the source are dropped silently.

## License

Apache 2.0 — see `LICENSE`.
