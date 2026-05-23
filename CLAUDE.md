# fx — Project Rules for Claude

This file lists the conventions specific to this repository. Cross-project
rules in `~/.claude/CLAUDE.md` and `~/.claude/rules/*.md` still apply; nothing
here overrides them unless explicitly stated.

## Core concept

`fx` is a format-exchange CLI. The non-obvious, load-bearing requirement is:

> **Preserve comments across formats whenever both sides can express them.**

That requirement is the reason the codebase uses an intermediate representation
(`internal/fxnode`) instead of converting through `map[string]any` — a plain Go
map loses both comments and key order. Keep this in mind when adding or
refactoring code: anything that funnels data through `any` is suspect.

## Package layout

```
main.go                       # entry point — calls cli.Run and exits
internal/
  cli/        # urfave/cli/v3 wiring. Exports Run(ctx, args, in, out, err)
  codec/      # Decoder/Encoder interfaces + per-format implementations
              # (json.go, yaml.go, toml.go, hcl.go, jsonnet.go)
  exchange/   # Exchange(src, dst, r, w) — pairs decoder & encoder
  format/     # Format type, extension map, content sniffing (Detect)
  fxnode/     # The IR: Node / Kind / MappingEntry / ScalarValue
  logging/    # clog-backed slog.Logger with context-scoped storage
```

Dependency direction: `cli → exchange → codec → fxnode + format`. The `format`
and `fxnode` packages have no internal dependencies.

`internal/` is used to block external import — `fx` is a CLI tool, not a
library.

## IR invariants (`internal/fxnode`)

- Exactly one of `Scalar` / `Mapping` / `Sequence` is meaningful per Node, keyed
  by `Kind`. The others are zero values.
- `Mapping` is a `[]MappingEntry` (not a map) so insertion order survives.
- Mapping keys are normally `KindScalar` nodes; codecs that genuinely need
  non-string keys may store other kinds, but every output format requires
  strings.
- Comments live as raw text in `HeadComment` / `LineComment` / `FootComment`,
  without the source format's comment markers (no leading `#`, `//`, etc.).
- Scalar values keep both a `Repr` (canonical text) and a `Tag` (semantic
  type). Encoders must respect the tag when choosing syntax.

## Adding a new format

1. Define the format constant and ext aliases in `internal/format/format.go`,
   plus a Detect heuristic if the format is content-identifiable.
2. Implement `Decoder` and `Encoder` (or just `Decoder` if it's decode-only)
   in a new file under `internal/codec/` named after the format.
3. Wire them in `internal/codec/codec.go`'s `DecoderFor` / `EncoderFor`.
4. Add a `testdata/{format}/` directory with at least `valid_*`, `comment_*`,
   and `invalid_*` samples.
5. Add per-format tests, plus a representative input to
   `internal/exchange/exchange_test.go`'s `representativeInputs` map so cross-
   format pairs get exercised.

## Comment-preservation matrix

| Source → Dest | JSON | YAML | TOML | HCL | Jsonnet |
|---------------|------|------|------|-----|---------|
| from JSON     | n/a  | (no in) | (no in) | (no in) | (no in) |
| from YAML     | drop | full | best-effort | best-effort | n/a |
| from TOML     | drop | best-effort | best-effort | best-effort | n/a |
| from HCL      | drop | best-effort | best-effort | best-effort | n/a |
| from Jsonnet  | drop | drop | drop | drop | n/a |

"Drop" is silent by design (no warning) — see spec for the rationale.

## Testing

- Coverage target: **100%** for every package under `internal/`. Gaps must be
  justified in a comment near the uncovered code.
- All test files use `package {name}_test`.
- Test data is local to the package that uses it (`internal/codec/testdata/…`),
  never in the repo root.
- `internal/exchange` carries representative cross-format inputs so a new
  codec is automatically exercised across all `(src, dst)` pairs.

## Notes for future changes

- HCL decoding currently lifts block labels via the special `_labels` mapping
  key. That key surfaces through cross-format conversion. If we ever change
  this representation, also update the spec, CLAUDE.md, and any tests that
  inspect `_labels`.
- TOML and HCL comment handling is acknowledged as best-effort. Strengthening
  it would require deeper coupling to the libraries' lower-level parsers.
- Jsonnet is decode-only on purpose — Jsonnet is an evaluation language, not a
  data format. Do not add an encoder.
