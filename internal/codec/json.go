package codec

import (
	"bytes"
	"encoding/json"
	"io"
	"strconv"

	"github.com/m-mizutani/fx/internal/fxnode"
	"github.com/m-mizutani/goerr/v2"
)

// jsonDecoder reads a JSON document into the IR. Object key ordering is
// preserved (encoding/json's MapSlice/Decoder token walk is used directly).
// Comments are not part of JSON's grammar so none are produced.
type jsonDecoder struct{}

func (jsonDecoder) Decode(r io.Reader) (*fxnode.Node, error) {
	dec := json.NewDecoder(r)
	dec.UseNumber()

	tok, err := dec.Token()
	if err != nil {
		return nil, goerr.Wrap(err, "read json token")
	}
	n, err := decodeJSONFromToken(dec, tok)
	if err != nil {
		return nil, goerr.Wrap(err, "decode json")
	}

	// Reject trailing content so we fail loudly on concatenated documents.
	if dec.More() {
		return nil, goerr.New("trailing content after top-level json value")
	}
	return n, nil
}

// decodeJSONValue reads the next JSON value from dec.
func decodeJSONValue(dec *json.Decoder) (*fxnode.Node, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	return decodeJSONFromToken(dec, tok)
}

// decodeJSONFromToken dispatches on an already-consumed token.
func decodeJSONFromToken(dec *json.Decoder, tok json.Token) (*fxnode.Node, error) {
	switch v := tok.(type) {
	case json.Delim:
		switch v {
		case '{':
			return decodeJSONObject(dec)
		case '[':
			return decodeJSONArray(dec)
		default:
			// `}` or `]` here is a structural error: a value was expected.
			return nil, goerr.New("unexpected closing delimiter", goerr.V("delim", string(v)))
		}
	case string:
		return fxnode.NewString(v), nil
	case json.Number:
		s := string(v)
		if _, err := strconv.ParseInt(s, 10, 64); err == nil {
			return fxnode.NewScalar(fxnode.TagInt, s), nil
		}
		return fxnode.NewScalar(fxnode.TagFloat, s), nil
	case bool:
		if v {
			return fxnode.NewScalar(fxnode.TagBool, "true"), nil
		}
		return fxnode.NewScalar(fxnode.TagBool, "false"), nil
	case nil:
		return fxnode.NewNull(), nil
	default:
		return nil, goerr.New("unknown json token", goerr.V("type", "unknown"))
	}
}

func decodeJSONObject(dec *json.Decoder) (*fxnode.Node, error) {
	m := fxnode.NewMapping()
	for dec.More() {
		ktok, err := dec.Token()
		if err != nil {
			return nil, goerr.Wrap(err, "read key")
		}
		key, ok := ktok.(string)
		if !ok {
			return nil, goerr.New("non-string key in json object")
		}
		val, err := decodeJSONValue(dec)
		if err != nil {
			return nil, goerr.Wrap(err, "decode value", goerr.V("key", key))
		}
		m.AddString(key, val)
	}
	// Consume closing `}`.
	if _, err := dec.Token(); err != nil {
		return nil, goerr.Wrap(err, "read closing brace")
	}
	return m, nil
}

func decodeJSONArray(dec *json.Decoder) (*fxnode.Node, error) {
	s := fxnode.NewSequence()
	for dec.More() {
		v, err := decodeJSONValue(dec)
		if err != nil {
			return nil, goerr.Wrap(err, "decode array element")
		}
		s.Append(v)
	}
	if _, err := dec.Token(); err != nil {
		return nil, goerr.Wrap(err, "read closing bracket")
	}
	return s, nil
}

// jsonEncoder writes the IR as pretty-printed JSON (two-space indent).
// Comments on IR nodes are silently dropped — JSON has no comment grammar.
type jsonEncoder struct{}

func (jsonEncoder) Encode(w io.Writer, n *fxnode.Node) error {
	var buf bytes.Buffer
	if err := encodeJSONNode(&buf, n, 0); err != nil {
		return goerr.Wrap(err, "encode json")
	}
	buf.WriteByte('\n')
	if _, err := w.Write(buf.Bytes()); err != nil {
		return goerr.Wrap(err, "write json")
	}
	return nil
}

func encodeJSONNode(buf *bytes.Buffer, n *fxnode.Node, depth int) error {
	switch n.Kind {
	case fxnode.KindScalar:
		return writeJSONScalar(buf, n.Scalar)
	case fxnode.KindMapping:
		return encodeJSONMapping(buf, n, depth)
	case fxnode.KindSequence:
		return encodeJSONSequence(buf, n, depth)
	default:
		return goerr.New("invalid node kind for json", goerr.V("kind", n.Kind.String()))
	}
}

func encodeJSONMapping(buf *bytes.Buffer, n *fxnode.Node, depth int) error {
	if len(n.Mapping) == 0 {
		buf.WriteString("{}")
		return nil
	}
	buf.WriteString("{\n")
	for i, e := range n.Mapping {
		writeIndent(buf, depth+1)
		if e.Key == nil || e.Key.Kind != fxnode.KindScalar {
			return goerr.New("mapping key must be a string scalar for json")
		}
		buf.WriteString(strconv.Quote(e.Key.Scalar.Repr))
		buf.WriteString(": ")
		if err := encodeJSONNode(buf, e.Value, depth+1); err != nil {
			return err
		}
		if i < len(n.Mapping)-1 {
			buf.WriteByte(',')
		}
		buf.WriteByte('\n')
	}
	writeIndent(buf, depth)
	buf.WriteByte('}')
	return nil
}

func encodeJSONSequence(buf *bytes.Buffer, n *fxnode.Node, depth int) error {
	if len(n.Sequence) == 0 {
		buf.WriteString("[]")
		return nil
	}
	buf.WriteString("[\n")
	for i, v := range n.Sequence {
		writeIndent(buf, depth+1)
		if err := encodeJSONNode(buf, v, depth+1); err != nil {
			return err
		}
		if i < len(n.Sequence)-1 {
			buf.WriteByte(',')
		}
		buf.WriteByte('\n')
	}
	writeIndent(buf, depth)
	buf.WriteByte(']')
	return nil
}

func writeJSONScalar(buf *bytes.Buffer, s fxnode.ScalarValue) error {
	switch s.Tag {
	case fxnode.TagString:
		buf.WriteString(strconv.Quote(s.Repr))
	case fxnode.TagInt, fxnode.TagFloat:
		buf.WriteString(s.Repr)
	case fxnode.TagBool:
		// Normalise to canonical lowercase form.
		if s.Repr == "true" {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case fxnode.TagNull:
		buf.WriteString("null")
	default:
		return goerr.New("invalid scalar tag for json", goerr.V("tag", s.Tag.String()))
	}
	return nil
}

// writeIndent writes the given indentation depth as two-space increments.
func writeIndent(buf *bytes.Buffer, depth int) {
	for i := 0; i < depth; i++ {
		buf.WriteString("  ")
	}
}
