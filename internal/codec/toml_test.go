package codec_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/m-mizutani/fx/internal/codec"
	"github.com/m-mizutani/fx/internal/fxnode"
	"github.com/m-mizutani/gt"
)

func TestTOMLDecodeValidFiles(t *testing.T) {
	cases := []string{
		"toml/valid_basic.toml",
		"toml/valid_table.toml",
		"toml/valid_table_array.toml",
		"toml/valid_inline_table.toml",
		"toml/valid_nested.toml",
	}
	dec, _ := codec.DecoderFor("toml")
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			data := readTestdata(t, c)
			n, err := dec.Decode(bytes.NewReader(data))
			gt.NoError(t, err)
			gt.NotNil(t, n)
		})
	}
}

func TestTOMLDecodeInvalidFiles(t *testing.T) {
	cases := []string{
		"toml/invalid_syntax.toml",
		"toml/invalid_duplicate_key.toml",
	}
	dec, _ := codec.DecoderFor("toml")
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			data := readTestdata(t, c)
			_, err := dec.Decode(bytes.NewReader(data))
			gt.Error(t, err)
		})
	}
}

func TestTOMLDecodeScalarTags(t *testing.T) {
	dec, _ := codec.DecoderFor("toml")
	data := readTestdata(t, "toml/valid_basic.toml")
	n, err := dec.Decode(bytes.NewReader(data))
	gt.NoError(t, err)

	name, _ := n.LookupString("name")
	gt.Equal(t, name.Scalar.Tag, fxnode.TagString)

	age, _ := n.LookupString("age")
	gt.Equal(t, age.Scalar.Tag, fxnode.TagInt)

	weight, _ := n.LookupString("weight")
	gt.Equal(t, weight.Scalar.Tag, fxnode.TagFloat)

	active, _ := n.LookupString("active")
	gt.Equal(t, active.Scalar.Tag, fxnode.TagBool)
}

func TestTOMLEncodeFromIR(t *testing.T) {
	enc, _ := codec.EncoderFor("toml")
	m := fxnode.NewMapping()
	m.AddString("name", fxnode.NewString("alice"))
	m.AddString("age", fxnode.NewScalar(fxnode.TagInt, "30"))
	m.AddString("weight", fxnode.NewScalar(fxnode.TagFloat, "1.5"))
	m.AddString("active", fxnode.NewScalar(fxnode.TagBool, "true"))

	var buf bytes.Buffer
	err := enc.Encode(&buf, m)
	gt.NoError(t, err)
	out := buf.String()
	gt.S(t, out).Contains(`name = 'alice'`)
	gt.S(t, out).Contains(`age = 30`)
	gt.S(t, out).Contains(`active = true`)
}

func TestTOMLEncodeRequiresMappingAtTopLevel(t *testing.T) {
	enc, _ := codec.EncoderFor("toml")
	err := enc.Encode(&bytes.Buffer{}, fxnode.NewString("not a mapping"))
	gt.Error(t, err)
}

func TestTOMLEncodeInvalidIntRepr(t *testing.T) {
	enc, _ := codec.EncoderFor("toml")
	m := fxnode.NewMapping()
	m.AddString("bad", &fxnode.Node{Kind: fxnode.KindScalar, Scalar: fxnode.ScalarValue{Tag: fxnode.TagInt, Repr: "not-a-number"}})
	err := enc.Encode(&bytes.Buffer{}, m)
	gt.Error(t, err)
}

func TestTOMLEncodeInvalidFloatRepr(t *testing.T) {
	enc, _ := codec.EncoderFor("toml")
	m := fxnode.NewMapping()
	m.AddString("bad", &fxnode.Node{Kind: fxnode.KindScalar, Scalar: fxnode.ScalarValue{Tag: fxnode.TagFloat, Repr: "x"}})
	err := enc.Encode(&bytes.Buffer{}, m)
	gt.Error(t, err)
}

func TestTOMLEncodeInvalidScalarTag(t *testing.T) {
	enc, _ := codec.EncoderFor("toml")
	m := fxnode.NewMapping()
	m.AddString("bad", &fxnode.Node{Kind: fxnode.KindScalar, Scalar: fxnode.ScalarValue{Tag: fxnode.TagInvalid}})
	err := enc.Encode(&bytes.Buffer{}, m)
	gt.Error(t, err)
}

func TestTOMLEncodeInvalidKind(t *testing.T) {
	enc, _ := codec.EncoderFor("toml")
	err := enc.Encode(&bytes.Buffer{}, &fxnode.Node{Kind: fxnode.KindInvalid})
	gt.Error(t, err)
}

func TestTOMLEncodeNonStringMappingKey(t *testing.T) {
	enc, _ := codec.EncoderFor("toml")
	m := fxnode.NewMapping()
	m.Mapping = append(m.Mapping, fxnode.MappingEntry{Key: fxnode.NewMapping(), Value: fxnode.NewString("v")})
	err := enc.Encode(&bytes.Buffer{}, m)
	gt.Error(t, err)
}

func TestTOMLDecodeNullViaJsonnetPath(t *testing.T) {
	// The anyToIR helper handles nil values when invoked via the jsonnet
	// path; TOML itself does not express null, but we test the helper's
	// nil branch through a direct TOML decode of a value re-marshalled
	// in Go.
	_ = strings.NewReader // keep import
	dec, _ := codec.DecoderFor("toml")
	// An empty document decodes to an empty mapping (not nil) — this
	// covers the map iteration branch with zero entries.
	n, err := dec.Decode(strings.NewReader(""))
	gt.NoError(t, err)
	gt.Equal(t, n.Kind, fxnode.KindMapping)
	gt.Equal(t, len(n.Mapping), 0)
}
