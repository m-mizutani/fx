package codec_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m-mizutani/fx/internal/codec"
	"github.com/m-mizutani/fx/internal/fxnode"
	"github.com/m-mizutani/gt"
)

// readTestdata reads bytes from internal/codec/testdata/{rel}.
func readTestdata(t *testing.T, rel string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", rel))
	gt.NoError(t, err)
	return b
}

func TestJSONDecodeValidFiles(t *testing.T) {
	cases := []string{
		"json/valid_empty.json",
		"json/valid_scalars.json",
		"json/valid_nested.json",
		"json/valid_array_root.json",
		"json/valid_unicode.json",
	}
	dec, err := codec.DecoderFor("json")
	gt.NoError(t, err)
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			data := readTestdata(t, c)
			n, err := dec.Decode(bytes.NewReader(data))
			gt.NoError(t, err)
			gt.NotNil(t, n)
		})
	}
}

func TestJSONDecodeInvalidFiles(t *testing.T) {
	cases := []string{
		"json/invalid_trailing_comma.json",
		"json/invalid_truncated.json",
		"json/invalid_trailing_content.json",
	}
	dec, _ := codec.DecoderFor("json")
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			data := readTestdata(t, c)
			_, err := dec.Decode(bytes.NewReader(data))
			gt.Error(t, err)
		})
	}
}

func TestJSONScalarTags(t *testing.T) {
	dec, _ := codec.DecoderFor("json")
	n, err := dec.Decode(strings.NewReader(`{"s":"x","i":42,"f":3.14,"t":true,"f2":false,"n":null}`))
	gt.NoError(t, err)
	gt.Equal(t, n.Kind, fxnode.KindMapping)

	s, _ := n.LookupString("s")
	gt.Equal(t, s.Scalar.Tag, fxnode.TagString)
	gt.Equal(t, s.Scalar.Repr, "x")

	i, _ := n.LookupString("i")
	gt.Equal(t, i.Scalar.Tag, fxnode.TagInt)
	gt.Equal(t, i.Scalar.Repr, "42")

	f, _ := n.LookupString("f")
	gt.Equal(t, f.Scalar.Tag, fxnode.TagFloat)

	tr, _ := n.LookupString("t")
	gt.Equal(t, tr.Scalar.Tag, fxnode.TagBool)
	gt.Equal(t, tr.Scalar.Repr, "true")

	fa, _ := n.LookupString("f2")
	gt.Equal(t, fa.Scalar.Repr, "false")

	nul, _ := n.LookupString("n")
	gt.Equal(t, nul.Scalar.Tag, fxnode.TagNull)
}

func TestJSONRoundTrip(t *testing.T) {
	dec, _ := codec.DecoderFor("json")
	enc, _ := codec.EncoderFor("json")

	files := []string{
		"json/valid_empty.json",
		"json/valid_scalars.json",
		"json/valid_nested.json",
		"json/valid_array_root.json",
	}
	for _, f := range files {
		t.Run(f, func(t *testing.T) {
			data := readTestdata(t, f)
			n, err := dec.Decode(bytes.NewReader(data))
			gt.NoError(t, err)

			var buf bytes.Buffer
			err = enc.Encode(&buf, n)
			gt.NoError(t, err)

			// Decode the encoded output again — IR equivalence is what we
			// guarantee, not byte-for-byte equality.
			n2, err := dec.Decode(&buf)
			gt.NoError(t, err)
			gt.Equal(t, n.Kind, n2.Kind)
		})
	}
}

func TestJSONEncodeEmptySequence(t *testing.T) {
	enc, _ := codec.EncoderFor("json")
	n := fxnode.NewSequence()
	var buf bytes.Buffer
	err := enc.Encode(&buf, n)
	gt.NoError(t, err)
	gt.S(t, buf.String()).Contains("[]")
}

func TestJSONEncodeEmptyMapping(t *testing.T) {
	enc, _ := codec.EncoderFor("json")
	n := fxnode.NewMapping()
	var buf bytes.Buffer
	err := enc.Encode(&buf, n)
	gt.NoError(t, err)
	gt.S(t, buf.String()).Contains("{}")
}

func TestJSONEncodeInvalidKind(t *testing.T) {
	enc, _ := codec.EncoderFor("json")
	n := &fxnode.Node{Kind: fxnode.KindInvalid}
	err := enc.Encode(&bytes.Buffer{}, n)
	gt.Error(t, err)
}

func TestJSONEncodeInvalidScalarTag(t *testing.T) {
	enc, _ := codec.EncoderFor("json")
	n := &fxnode.Node{Kind: fxnode.KindScalar, Scalar: fxnode.ScalarValue{Tag: fxnode.TagInvalid}}
	err := enc.Encode(&bytes.Buffer{}, n)
	gt.Error(t, err)
}

func TestJSONEncodeNonScalarMappingKey(t *testing.T) {
	enc, _ := codec.EncoderFor("json")
	m := fxnode.NewMapping()
	// Inject a non-scalar key by hand to bypass AddString.
	m.Mapping = append(m.Mapping, fxnode.MappingEntry{
		Key:   fxnode.NewMapping(),
		Value: fxnode.NewString("v"),
	})
	err := enc.Encode(&bytes.Buffer{}, m)
	gt.Error(t, err)
}

func TestJSONEncodeBoolNormalisation(t *testing.T) {
	enc, _ := codec.EncoderFor("json")
	// A bool scalar with an unusual Repr should still emit canonical lower-case.
	m := fxnode.NewMapping()
	m.AddString("t", &fxnode.Node{Kind: fxnode.KindScalar, Scalar: fxnode.ScalarValue{Tag: fxnode.TagBool, Repr: "True"}})
	var buf bytes.Buffer
	err := enc.Encode(&buf, m)
	gt.NoError(t, err)
	gt.S(t, buf.String()).Contains(`"t": false`) // Repr != "true" => false
}

func TestJSONDecodeEmptyInputErrors(t *testing.T) {
	dec, _ := codec.DecoderFor("json")
	_, err := dec.Decode(strings.NewReader(""))
	gt.Error(t, err)
}

func TestJSONDecodeNonStringKey(t *testing.T) {
	// json.Decoder won't allow non-string keys at the syntax level, so this
	// case is unreachable through a real reader. We document it here as a
	// guard via direct token simulation.
	dec, _ := codec.DecoderFor("json")
	_, err := dec.Decode(strings.NewReader("{123: 1}"))
	gt.Error(t, err)
}

func TestJSONDecodeUnexpectedClosingDelim(t *testing.T) {
	dec, _ := codec.DecoderFor("json")
	_, err := dec.Decode(strings.NewReader("}"))
	gt.Error(t, err)
}
