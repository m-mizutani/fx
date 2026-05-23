package codec_test

import (
	"bytes"
	"testing"

	"github.com/m-mizutani/fx/internal/codec"
	"github.com/m-mizutani/fx/internal/fxnode"
	"github.com/m-mizutani/gt"
)

func TestHCLDecodeValidFiles(t *testing.T) {
	cases := []string{
		"hcl/valid_attribute_only.hcl",
		"hcl/valid_block.hcl",
		"hcl/valid_nested_block.hcl",
		"hcl/valid_list_and_object.hcl",
		"hcl/valid_multi_block_same_type.hcl",
	}
	dec, _ := codec.DecoderFor("hcl")
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			data := readTestdata(t, c)
			n, err := dec.Decode(bytes.NewReader(data))
			gt.NoError(t, err)
			gt.NotNil(t, n)
		})
	}
}

func TestHCLDecodeInvalidFiles(t *testing.T) {
	cases := []string{
		"hcl/invalid_unclosed_block.hcl",
		"hcl/invalid_dangling_attr.hcl",
	}
	dec, _ := codec.DecoderFor("hcl")
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			data := readTestdata(t, c)
			_, err := dec.Decode(bytes.NewReader(data))
			gt.Error(t, err)
		})
	}
}

func TestHCLDecodeScalarTags(t *testing.T) {
	dec, _ := codec.DecoderFor("hcl")
	data := readTestdata(t, "hcl/valid_attribute_only.hcl")
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

	nick, _ := n.LookupString("nick")
	gt.Equal(t, nick.Scalar.Tag, fxnode.TagNull)
}

func TestHCLDecodeBlockLabels(t *testing.T) {
	dec, _ := codec.DecoderFor("hcl")
	data := readTestdata(t, "hcl/valid_block.hcl")
	n, err := dec.Decode(bytes.NewReader(data))
	gt.NoError(t, err)

	server, _ := n.LookupString("server")
	gt.NotNil(t, server)
	labels, _ := server.LookupString("_labels")
	gt.NotNil(t, labels)
	gt.Equal(t, labels.Kind, fxnode.KindSequence)
	gt.Equal(t, len(labels.Sequence), 1)
	gt.Equal(t, labels.Sequence[0].Scalar.Repr, "primary")
}

func TestHCLEncodeFromIR(t *testing.T) {
	enc, _ := codec.EncoderFor("hcl")
	m := fxnode.NewMapping()
	m.AddString("name", fxnode.NewString("alice"))
	m.AddString("age", fxnode.NewScalar(fxnode.TagInt, "30"))
	m.AddString("weight", fxnode.NewScalar(fxnode.TagFloat, "1.5"))
	m.AddString("active", fxnode.NewScalar(fxnode.TagBool, "true"))
	m.AddString("nick", fxnode.NewNull())
	m.AddString("tags", func() *fxnode.Node {
		s := fxnode.NewSequence()
		s.Append(fxnode.NewString("a"))
		s.Append(fxnode.NewString("b"))
		return s
	}())
	m.AddString("nested", func() *fxnode.Node {
		inner := fxnode.NewMapping()
		inner.AddString("k", fxnode.NewString("v"))
		return inner
	}())

	var buf bytes.Buffer
	err := enc.Encode(&buf, m)
	gt.NoError(t, err)
	out := buf.String()
	gt.S(t, out).Contains("name")
	gt.S(t, out).Contains(`"alice"`)
}

func TestHCLEncodeEmptyMapping(t *testing.T) {
	enc, _ := codec.EncoderFor("hcl")
	m := fxnode.NewMapping()
	m.AddString("nested", fxnode.NewMapping())
	m.AddString("list", fxnode.NewSequence())
	var buf bytes.Buffer
	err := enc.Encode(&buf, m)
	gt.NoError(t, err)
}

func TestHCLEncodeRequiresMappingAtTopLevel(t *testing.T) {
	enc, _ := codec.EncoderFor("hcl")
	err := enc.Encode(&bytes.Buffer{}, fxnode.NewString("x"))
	gt.Error(t, err)
}

func TestHCLEncodeNonStringMappingKey(t *testing.T) {
	enc, _ := codec.EncoderFor("hcl")
	m := fxnode.NewMapping()
	m.Mapping = append(m.Mapping, fxnode.MappingEntry{Key: fxnode.NewMapping(), Value: fxnode.NewString("v")})
	err := enc.Encode(&bytes.Buffer{}, m)
	gt.Error(t, err)
}

func TestHCLEncodeInvalidKind(t *testing.T) {
	enc, _ := codec.EncoderFor("hcl")
	m := fxnode.NewMapping()
	m.AddString("bad", &fxnode.Node{Kind: fxnode.KindInvalid})
	err := enc.Encode(&bytes.Buffer{}, m)
	gt.Error(t, err)
}

func TestHCLEncodeInvalidIntRepr(t *testing.T) {
	enc, _ := codec.EncoderFor("hcl")
	m := fxnode.NewMapping()
	m.AddString("bad", &fxnode.Node{Kind: fxnode.KindScalar, Scalar: fxnode.ScalarValue{Tag: fxnode.TagInt, Repr: "x"}})
	err := enc.Encode(&bytes.Buffer{}, m)
	gt.Error(t, err)
}

func TestHCLEncodeInvalidFloatRepr(t *testing.T) {
	enc, _ := codec.EncoderFor("hcl")
	m := fxnode.NewMapping()
	m.AddString("bad", &fxnode.Node{Kind: fxnode.KindScalar, Scalar: fxnode.ScalarValue{Tag: fxnode.TagFloat, Repr: "x"}})
	err := enc.Encode(&bytes.Buffer{}, m)
	gt.Error(t, err)
}

func TestHCLEncodeInvalidScalarTag(t *testing.T) {
	enc, _ := codec.EncoderFor("hcl")
	m := fxnode.NewMapping()
	m.AddString("bad", &fxnode.Node{Kind: fxnode.KindScalar, Scalar: fxnode.ScalarValue{Tag: fxnode.TagInvalid}})
	err := enc.Encode(&bytes.Buffer{}, m)
	gt.Error(t, err)
}

func TestHCLEncodeNestedNonStringMappingKey(t *testing.T) {
	enc, _ := codec.EncoderFor("hcl")
	m := fxnode.NewMapping()
	inner := fxnode.NewMapping()
	inner.Mapping = append(inner.Mapping, fxnode.MappingEntry{Key: fxnode.NewMapping(), Value: fxnode.NewString("v")})
	m.AddString("outer", inner)
	err := enc.Encode(&bytes.Buffer{}, m)
	gt.Error(t, err)
}
