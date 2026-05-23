package codec

import (
	"bytes"
	"io"
	"sort"
	"strconv"

	"github.com/m-mizutani/fx/internal/fxnode"
	"github.com/m-mizutani/goerr/v2"
	"github.com/pelletier/go-toml/v2"
)

// tomlDecoder reads TOML using go-toml/v2's Unmarshal into a generic
// map[string]any tree and then converts it into the IR.
//
// Limitations (documented in spec):
//   - Mapping order is not preserved: Go maps have no order, so we sort keys
//     alphabetically for deterministic output.
//   - Comments are dropped (go-toml/v2's stable API does not expose them).
type tomlDecoder struct{}

func (tomlDecoder) Decode(r io.Reader) (*fxnode.Node, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, goerr.Wrap(err, "read toml")
	}
	var raw map[string]any
	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil, goerr.Wrap(err, "unmarshal toml")
	}
	return anyToIR(raw)
}

// anyToIR converts a value decoded by encoding-style libraries (toml/jsonnet
// after JSON re-encode) into the IR. It accepts the common Go types produced
// by such libraries and is used by both TOML and Jsonnet decoders.
func anyToIR(v any) (*fxnode.Node, error) {
	switch x := v.(type) {
	case nil:
		return fxnode.NewNull(), nil
	case string:
		return fxnode.NewString(x), nil
	case bool:
		if x {
			return fxnode.NewScalar(fxnode.TagBool, "true"), nil
		}
		return fxnode.NewScalar(fxnode.TagBool, "false"), nil
	case int:
		return fxnode.NewScalar(fxnode.TagInt, strconv.FormatInt(int64(x), 10)), nil
	case int64:
		return fxnode.NewScalar(fxnode.TagInt, strconv.FormatInt(x, 10)), nil
	case uint64:
		return fxnode.NewScalar(fxnode.TagInt, strconv.FormatUint(x, 10)), nil
	case float64:
		return fxnode.NewScalar(fxnode.TagFloat, strconv.FormatFloat(x, 'g', -1, 64)), nil
	case []any:
		s := fxnode.NewSequence()
		for _, e := range x {
			ch, err := anyToIR(e)
			if err != nil {
				return nil, err
			}
			s.Append(ch)
		}
		return s, nil
	case map[string]any:
		m := fxnode.NewMapping()
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			ch, err := anyToIR(x[k])
			if err != nil {
				return nil, err
			}
			m.AddString(k, ch)
		}
		return m, nil
	default:
		// Fall back to fmt-formatted string for less common types (datetime
		// values from go-toml stringify cleanly via their String()).
		if s, ok := v.(interface{ String() string }); ok {
			return fxnode.NewString(s.String()), nil
		}
		return nil, goerr.New("unsupported value type for ir", goerr.V("type", "unknown"))
	}
}

// tomlEncoder writes IR as TOML using go-toml/v2's Marshal. The IR is first
// converted to a generic interface tree because go-toml has no public API for
// custom-ordered mappings.
type tomlEncoder struct{}

func (tomlEncoder) Encode(w io.Writer, n *fxnode.Node) error {
	v, err := irToAny(n)
	if err != nil {
		return goerr.Wrap(err, "convert ir for toml")
	}
	m, ok := v.(map[string]any)
	if !ok {
		// TOML requires a table at the top level.
		return goerr.New("top-level node must be a mapping for toml output",
			goerr.V("kind", n.Kind.String()))
	}
	var buf bytes.Buffer
	enc := toml.NewEncoder(&buf)
	if err := enc.Encode(m); err != nil {
		return goerr.Wrap(err, "encode toml")
	}
	if _, err := w.Write(buf.Bytes()); err != nil {
		return goerr.Wrap(err, "write toml")
	}
	return nil
}

// irToAny converts the IR back into the standard Go types accepted by
// general-purpose encoders. Scalars are emitted with their semantic type
// (int as int64, float as float64, …) so that the target encoder produces
// the expected syntax.
func irToAny(n *fxnode.Node) (any, error) {
	switch n.Kind {
	case fxnode.KindScalar:
		return scalarToAny(n.Scalar)
	case fxnode.KindMapping:
		m := make(map[string]any, len(n.Mapping))
		for _, e := range n.Mapping {
			if e.Key == nil || e.Key.Kind != fxnode.KindScalar {
				return nil, goerr.New("mapping key must be a string scalar")
			}
			v, err := irToAny(e.Value)
			if err != nil {
				return nil, err
			}
			m[e.Key.Scalar.Repr] = v
		}
		return m, nil
	case fxnode.KindSequence:
		s := make([]any, 0, len(n.Sequence))
		for _, c := range n.Sequence {
			v, err := irToAny(c)
			if err != nil {
				return nil, err
			}
			s = append(s, v)
		}
		return s, nil
	default:
		return nil, goerr.New("invalid kind", goerr.V("kind", n.Kind.String()))
	}
}

func scalarToAny(s fxnode.ScalarValue) (any, error) {
	switch s.Tag {
	case fxnode.TagString:
		return s.Repr, nil
	case fxnode.TagInt:
		i, err := strconv.ParseInt(s.Repr, 10, 64)
		if err != nil {
			return nil, goerr.Wrap(err, "parse int scalar", goerr.V("repr", s.Repr))
		}
		return i, nil
	case fxnode.TagFloat:
		f, err := strconv.ParseFloat(s.Repr, 64)
		if err != nil {
			return nil, goerr.Wrap(err, "parse float scalar", goerr.V("repr", s.Repr))
		}
		return f, nil
	case fxnode.TagBool:
		return s.Repr == "true", nil
	case fxnode.TagNull:
		return nil, nil
	default:
		return nil, goerr.New("invalid scalar tag", goerr.V("tag", s.Tag.String()))
	}
}
