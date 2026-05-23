package codec_test

import (
	"bytes"
	"testing"

	"github.com/m-mizutani/fx/internal/codec"
	"github.com/m-mizutani/fx/internal/fxnode"
	"github.com/m-mizutani/gt"
)

func TestJsonnetDecodeValidFiles(t *testing.T) {
	cases := []string{
		"jsonnet/valid_object.jsonnet",
		"jsonnet/valid_local.jsonnet",
		"jsonnet/valid_arithmetic.jsonnet",
		"jsonnet/valid_function_call.jsonnet",
	}
	dec, _ := codec.DecoderFor("jsonnet")
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			data := readTestdata(t, c)
			n, err := dec.Decode(bytes.NewReader(data))
			gt.NoError(t, err)
			gt.NotNil(t, n)
		})
	}
}

func TestJsonnetDecodeInvalidFiles(t *testing.T) {
	cases := []string{
		"jsonnet/invalid_syntax.jsonnet",
		"jsonnet/invalid_eval_error.jsonnet",
		"jsonnet/invalid_function_root.jsonnet",
	}
	dec, _ := codec.DecoderFor("jsonnet")
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			data := readTestdata(t, c)
			_, err := dec.Decode(bytes.NewReader(data))
			gt.Error(t, err)
		})
	}
}

func TestJsonnetDecodeResultStructure(t *testing.T) {
	dec, _ := codec.DecoderFor("jsonnet")
	data := readTestdata(t, "jsonnet/valid_object.jsonnet")
	n, err := dec.Decode(bytes.NewReader(data))
	gt.NoError(t, err)
	gt.Equal(t, n.Kind, fxnode.KindMapping)

	name, ok := n.LookupString("name")
	gt.True(t, ok)
	gt.Equal(t, name.Scalar.Repr, "alice")
}
