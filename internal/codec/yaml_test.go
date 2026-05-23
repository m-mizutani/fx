package codec_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/m-mizutani/fx/internal/codec"
	"github.com/m-mizutani/fx/internal/fxnode"
	"github.com/m-mizutani/gt"
)

func TestYAMLDecodeValidFiles(t *testing.T) {
	cases := []string{
		"yaml/valid_empty.yaml",
		"yaml/valid_scalars.yaml",
		"yaml/valid_nested.yaml",
		"yaml/valid_flow_style.yaml",
		"yaml/valid_quoted_strings.yaml",
		"yaml/valid_anchor_alias.yaml",
	}
	dec, _ := codec.DecoderFor("yaml")
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			data := readTestdata(t, c)
			n, err := dec.Decode(bytes.NewReader(data))
			gt.NoError(t, err)
			gt.NotNil(t, n)
		})
	}
}

func TestYAMLDecodeInvalidFiles(t *testing.T) {
	cases := []string{
		"yaml/invalid_indent.yaml",
		"yaml/invalid_tab.yaml",
		"yaml/invalid_multidoc.yaml",
	}
	dec, _ := codec.DecoderFor("yaml")
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			data := readTestdata(t, c)
			_, err := dec.Decode(bytes.NewReader(data))
			gt.Error(t, err)
		})
	}
}

func TestYAMLDecodeScalarTags(t *testing.T) {
	dec, _ := codec.DecoderFor("yaml")
	data := readTestdata(t, "yaml/valid_scalars.yaml")
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

	nick, _ := n.LookupString("nickname")
	gt.Equal(t, nick.Scalar.Tag, fxnode.TagNull)
}

func TestYAMLCommentRoundTrip(t *testing.T) {
	dec, _ := codec.DecoderFor("yaml")
	enc, _ := codec.EncoderFor("yaml")
	data := readTestdata(t, "yaml/comment_head.yaml")
	n, err := dec.Decode(bytes.NewReader(data))
	gt.NoError(t, err)

	var buf bytes.Buffer
	err = enc.Encode(&buf, n)
	gt.NoError(t, err)

	// Comments are preserved on output.
	gt.S(t, buf.String()).Contains("Server configuration block")
	gt.S(t, buf.String()).Contains("The port to listen on")
}

func TestYAMLLineCommentRoundTrip(t *testing.T) {
	dec, _ := codec.DecoderFor("yaml")
	enc, _ := codec.EncoderFor("yaml")
	data := readTestdata(t, "yaml/comment_line.yaml")
	n, err := dec.Decode(bytes.NewReader(data))
	gt.NoError(t, err)

	var buf bytes.Buffer
	gt.NoError(t, enc.Encode(&buf, n))
	gt.S(t, buf.String()).Contains("primary user")
	gt.S(t, buf.String()).Contains("in years")
}

func TestYAMLEncodeIRMappingAndSequence(t *testing.T) {
	enc, _ := codec.EncoderFor("yaml")
	m := fxnode.NewMapping()
	m.AddString("name", fxnode.NewString("bob"))
	m.AddString("tags", func() *fxnode.Node {
		s := fxnode.NewSequence()
		s.Append(fxnode.NewString("a"))
		s.Append(fxnode.NewString("b"))
		return s
	}())
	m.AddString("count", fxnode.NewScalar(fxnode.TagInt, "5"))
	m.AddString("ratio", fxnode.NewScalar(fxnode.TagFloat, "1.5"))
	m.AddString("active", fxnode.NewScalar(fxnode.TagBool, "true"))
	m.AddString("nick", fxnode.NewNull())

	var buf bytes.Buffer
	err := enc.Encode(&buf, m)
	gt.NoError(t, err)
	out := buf.String()
	gt.S(t, out).Contains("name:")
	gt.S(t, out).Contains("tags:")
	gt.S(t, out).Contains("count:")
}

func TestYAMLEncodeInvalidKind(t *testing.T) {
	enc, _ := codec.EncoderFor("yaml")
	err := enc.Encode(&bytes.Buffer{}, &fxnode.Node{Kind: fxnode.KindInvalid})
	gt.Error(t, err)
}

func TestYAMLEmptyDocument(t *testing.T) {
	dec, _ := codec.DecoderFor("yaml")
	n, err := dec.Decode(strings.NewReader(""))
	gt.NoError(t, err)
	gt.Equal(t, n.Kind, fxnode.KindScalar)
	gt.Equal(t, n.Scalar.Tag, fxnode.TagNull)
}

func TestYAMLExplicitTags(t *testing.T) {
	dec, _ := codec.DecoderFor("yaml")
	n, err := dec.Decode(strings.NewReader("v: !!str 1\nf: !!float 1\nb: !!bool true\nz: !!null \n"))
	gt.NoError(t, err)

	v, _ := n.LookupString("v")
	gt.Equal(t, v.Scalar.Tag, fxnode.TagString)
	gt.Equal(t, v.Scalar.Repr, "1")

	f, _ := n.LookupString("f")
	gt.Equal(t, f.Scalar.Tag, fxnode.TagFloat)

	b, _ := n.LookupString("b")
	gt.Equal(t, b.Scalar.Tag, fxnode.TagBool)

	z, _ := n.LookupString("z")
	gt.Equal(t, z.Scalar.Tag, fxnode.TagNull)
}

func TestYAMLEncodeAllScalarTags(t *testing.T) {
	enc, _ := codec.EncoderFor("yaml")
	m := fxnode.NewMapping()
	m.AddString("default_tag", &fxnode.Node{Kind: fxnode.KindScalar, Scalar: fxnode.ScalarValue{Tag: fxnode.TagInvalid, Repr: "x"}})
	var buf bytes.Buffer
	err := enc.Encode(&buf, m)
	gt.NoError(t, err)
	gt.S(t, buf.String()).Contains("default_tag")
}
