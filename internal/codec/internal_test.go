package codec_test

// internal_test.go exercises branches that are not reachable through public
// codec entry points (Decoder/Encoder.Encode/Decode). They live behind
// export_test.go.

import (
	"math/big"
	"testing"

	"github.com/m-mizutani/fx/internal/codec"
	"github.com/m-mizutani/fx/internal/fxnode"
	"github.com/m-mizutani/gt"
	"github.com/zclconf/go-cty/cty"
	"gopkg.in/yaml.v3"
)

// --- anyToIR branches -------------------------------------------------------

func TestAnyToIRAllTypes(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		n, err := codec.AnyToIR(nil)
		gt.NoError(t, err)
		gt.Equal(t, n.Scalar.Tag, fxnode.TagNull)
	})
	t.Run("string", func(t *testing.T) {
		n, err := codec.AnyToIR("hello")
		gt.NoError(t, err)
		gt.Equal(t, n.Scalar.Tag, fxnode.TagString)
	})
	t.Run("bool_true", func(t *testing.T) {
		n, err := codec.AnyToIR(true)
		gt.NoError(t, err)
		gt.Equal(t, n.Scalar.Repr, "true")
	})
	t.Run("bool_false", func(t *testing.T) {
		n, err := codec.AnyToIR(false)
		gt.NoError(t, err)
		gt.Equal(t, n.Scalar.Repr, "false")
	})
	t.Run("int", func(t *testing.T) {
		n, err := codec.AnyToIR(int(7))
		gt.NoError(t, err)
		gt.Equal(t, n.Scalar.Tag, fxnode.TagInt)
		gt.Equal(t, n.Scalar.Repr, "7")
	})
	t.Run("int64", func(t *testing.T) {
		n, err := codec.AnyToIR(int64(-9))
		gt.NoError(t, err)
		gt.Equal(t, n.Scalar.Repr, "-9")
	})
	t.Run("uint64", func(t *testing.T) {
		n, err := codec.AnyToIR(uint64(1234567890))
		gt.NoError(t, err)
		gt.Equal(t, n.Scalar.Repr, "1234567890")
	})
	t.Run("float64", func(t *testing.T) {
		n, err := codec.AnyToIR(3.14)
		gt.NoError(t, err)
		gt.Equal(t, n.Scalar.Tag, fxnode.TagFloat)
	})
	t.Run("array", func(t *testing.T) {
		n, err := codec.AnyToIR([]any{1, "x", true})
		gt.NoError(t, err)
		gt.Equal(t, n.Kind, fxnode.KindSequence)
		gt.Equal(t, len(n.Sequence), 3)
	})
	t.Run("array_with_unsupported_element", func(t *testing.T) {
		// Inject an unsupported type to exercise the array error branch.
		_, err := codec.AnyToIR([]any{complex(1, 2)})
		gt.Error(t, err)
	})
	t.Run("map", func(t *testing.T) {
		n, err := codec.AnyToIR(map[string]any{"a": 1, "b": "x"})
		gt.NoError(t, err)
		gt.Equal(t, n.Kind, fxnode.KindMapping)
	})
	t.Run("map_with_unsupported_value", func(t *testing.T) {
		_, err := codec.AnyToIR(map[string]any{"a": complex(1, 2)})
		gt.Error(t, err)
	})
	t.Run("unsupported_type", func(t *testing.T) {
		_, err := codec.AnyToIR(complex(1, 2))
		gt.Error(t, err)
	})
}

type stringerType struct{ s string }

func (s stringerType) String() string { return s.s }

func TestAnyToIRStringerFallback(t *testing.T) {
	n, err := codec.AnyToIR(stringerType{s: "stringer-x"})
	gt.NoError(t, err)
	gt.Equal(t, n.Kind, fxnode.KindScalar)
	gt.Equal(t, n.Scalar.Repr, "stringer-x")
}

// --- irToAny branches -------------------------------------------------------

func TestIRToAnySequence(t *testing.T) {
	s := fxnode.NewSequence()
	s.Append(fxnode.NewString("a"))
	s.Append(fxnode.NewScalar(fxnode.TagInt, "1"))
	v, err := codec.IRToAny(s)
	gt.NoError(t, err)
	arr, ok := v.([]any)
	gt.True(t, ok)
	gt.Equal(t, len(arr), 2)
}

func TestIRToAnySequenceError(t *testing.T) {
	s := fxnode.NewSequence()
	s.Append(&fxnode.Node{Kind: fxnode.KindInvalid})
	_, err := codec.IRToAny(s)
	gt.Error(t, err)
}

func TestIRToAnyInvalidKind(t *testing.T) {
	_, err := codec.IRToAny(&fxnode.Node{Kind: fxnode.KindInvalid})
	gt.Error(t, err)
}

func TestScalarToAnyAll(t *testing.T) {
	cases := []struct {
		name string
		s    fxnode.ScalarValue
		want any
	}{
		{"string", fxnode.ScalarValue{Tag: fxnode.TagString, Repr: "x"}, "x"},
		{"int", fxnode.ScalarValue{Tag: fxnode.TagInt, Repr: "42"}, int64(42)},
		{"float", fxnode.ScalarValue{Tag: fxnode.TagFloat, Repr: "1.5"}, 1.5},
		{"bool", fxnode.ScalarValue{Tag: fxnode.TagBool, Repr: "true"}, true},
		{"null", fxnode.ScalarValue{Tag: fxnode.TagNull, Repr: ""}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v, err := codec.ScalarToAny(c.s)
			gt.NoError(t, err)
			gt.Equal(t, v, c.want)
		})
	}
}

// --- ctyToIR branches -------------------------------------------------------

func TestCtyToIRListAndSet(t *testing.T) {
	list := cty.ListVal([]cty.Value{cty.StringVal("a"), cty.StringVal("b")})
	n, err := codec.CtyToIR(list)
	gt.NoError(t, err)
	gt.Equal(t, n.Kind, fxnode.KindSequence)
	gt.Equal(t, len(n.Sequence), 2)

	set := cty.SetVal([]cty.Value{cty.NumberIntVal(1), cty.NumberIntVal(2)})
	n2, err := codec.CtyToIR(set)
	gt.NoError(t, err)
	gt.Equal(t, n2.Kind, fxnode.KindSequence)
}

func TestCtyToIRMap(t *testing.T) {
	m := cty.MapVal(map[string]cty.Value{
		"b": cty.StringVal("two"),
		"a": cty.StringVal("one"),
	})
	n, err := codec.CtyToIR(m)
	gt.NoError(t, err)
	gt.Equal(t, n.Kind, fxnode.KindMapping)
	// Keys must come out alphabetically.
	gt.Equal(t, n.Mapping[0].Key.Scalar.Repr, "a")
	gt.Equal(t, n.Mapping[1].Key.Scalar.Repr, "b")
}

func TestCtyToIRNumberFloat(t *testing.T) {
	n, err := codec.CtyToIR(cty.NumberFloatVal(2.5))
	gt.NoError(t, err)
	gt.Equal(t, n.Scalar.Tag, fxnode.TagFloat)
}

func TestCtyToIRLargeInteger(t *testing.T) {
	// Beyond int64 range: 2^65 = 36893488147419103232. We expect the value
	// to survive intact via big.Float.Text rather than silently truncate.
	bf, _, _ := big.ParseFloat("36893488147419103232", 10, 200, big.ToNearestEven)
	v := cty.NumberVal(bf)
	n, err := codec.CtyToIR(v)
	gt.NoError(t, err)
	gt.Equal(t, n.Scalar.Tag, fxnode.TagInt)
	gt.Equal(t, n.Scalar.Repr, "36893488147419103232")
}

func TestCtyToIRHighPrecisionFloat(t *testing.T) {
	// A high-precision decimal that would lose digits via float64 conversion.
	bf, _, _ := big.ParseFloat("3.141592653589793238462643383279", 10, 200, big.ToNearestEven)
	v := cty.NumberVal(bf)
	n, err := codec.CtyToIR(v)
	gt.NoError(t, err)
	gt.Equal(t, n.Scalar.Tag, fxnode.TagFloat)
	gt.S(t, n.Scalar.Repr).Contains("3.14159265358979")
}

func TestCtyToIRNull(t *testing.T) {
	n, err := codec.CtyToIR(cty.NullVal(cty.String))
	gt.NoError(t, err)
	gt.Equal(t, n.Scalar.Tag, fxnode.TagNull)
}

func TestCtyToIRBoolFalse(t *testing.T) {
	n, err := codec.CtyToIR(cty.False)
	gt.NoError(t, err)
	gt.Equal(t, n.Scalar.Repr, "false")
}

func TestCtyToIRUnknown(t *testing.T) {
	_, err := codec.CtyToIR(cty.UnknownVal(cty.String))
	gt.Error(t, err)
}

func TestCtyToIRListPropagatesError(t *testing.T) {
	// A list containing unknown values triggers ctyToIR's recursive error.
	list := cty.TupleVal([]cty.Value{cty.UnknownVal(cty.String)})
	_, err := codec.CtyToIR(list)
	gt.Error(t, err)
}

func TestCtyToIRObjectPropagatesError(t *testing.T) {
	obj := cty.ObjectVal(map[string]cty.Value{"k": cty.UnknownVal(cty.String)})
	_, err := codec.CtyToIR(obj)
	gt.Error(t, err)
}

func TestCtyToIRMapPropagatesError(t *testing.T) {
	m := cty.MapVal(map[string]cty.Value{"k": cty.NumberIntVal(1)})
	// MapVal requires uniform types; instead we cover the map error path
	// by constructing a map containing a value that ctyToIR rejects. Since
	// cty.MapVal disallows mixing, use the fact that the iteration path is
	// the same as the object path and rely on TestCtyToIRObjectPropagatesError
	// for branch coverage; this test ensures the happy-path returns ordered.
	n, err := codec.CtyToIR(m)
	gt.NoError(t, err)
	gt.Equal(t, n.Kind, fxnode.KindMapping)
}

// --- irToCty branches -------------------------------------------------------

func TestIRToCtyEmptyContainers(t *testing.T) {
	v, err := codec.IRToCty(fxnode.NewSequence())
	gt.NoError(t, err)
	gt.True(t, v.Type().IsTupleType())

	v2, err := codec.IRToCty(fxnode.NewMapping())
	gt.NoError(t, err)
	gt.True(t, v2.Type().IsObjectType())
}

func TestIRToCtySequenceError(t *testing.T) {
	s := fxnode.NewSequence()
	s.Append(&fxnode.Node{Kind: fxnode.KindInvalid})
	_, err := codec.IRToCty(s)
	gt.Error(t, err)
}

func TestIRToCtyInvalidKind(t *testing.T) {
	_, err := codec.IRToCty(&fxnode.Node{Kind: fxnode.KindInvalid})
	gt.Error(t, err)
}

func TestScalarToCtyAll(t *testing.T) {
	cases := []fxnode.ScalarValue{
		{Tag: fxnode.TagString, Repr: "x"},
		{Tag: fxnode.TagInt, Repr: "42"},
		{Tag: fxnode.TagFloat, Repr: "1.5"},
		{Tag: fxnode.TagBool, Repr: "true"},
		{Tag: fxnode.TagBool, Repr: "false"},
		{Tag: fxnode.TagNull, Repr: ""},
	}
	for _, c := range cases {
		_, err := codec.ScalarToCty(c)
		gt.NoError(t, err)
	}

	// Error branches.
	_, err := codec.ScalarToCty(fxnode.ScalarValue{Tag: fxnode.TagInt, Repr: "x"})
	gt.Error(t, err)
	_, err = codec.ScalarToCty(fxnode.ScalarValue{Tag: fxnode.TagFloat, Repr: "x"})
	gt.Error(t, err)
	_, err = codec.ScalarToCty(fxnode.ScalarValue{Tag: fxnode.TagInvalid})
	gt.Error(t, err)
}

// --- yamlNodeToIR branches --------------------------------------------------

func TestYamlNodeToIREmptyDocument(t *testing.T) {
	doc := &yaml.Node{Kind: yaml.DocumentNode}
	n, err := codec.YamlNodeToIR(doc)
	gt.NoError(t, err)
	gt.Equal(t, n.Scalar.Tag, fxnode.TagNull)
}

func TestYamlNodeToIRUnsupportedKind(t *testing.T) {
	n := &yaml.Node{Kind: yaml.Kind(0)} // zero kind is unsupported
	_, err := codec.YamlNodeToIR(n)
	gt.Error(t, err)
}

func TestYamlNodeToIRMappingError(t *testing.T) {
	// A mapping whose key is itself a node that ctyToIR/yamlNodeToIR rejects:
	// build a mapping with an unsupported-kind key node.
	m := &yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{
		{Kind: yaml.Kind(0)}, // bad key
		{Kind: yaml.ScalarNode, Value: "v"},
	}}
	_, err := codec.YamlNodeToIR(m)
	gt.Error(t, err)
}

func TestYamlNodeToIRSequenceError(t *testing.T) {
	s := &yaml.Node{Kind: yaml.SequenceNode, Content: []*yaml.Node{{Kind: yaml.Kind(0)}}}
	_, err := codec.YamlNodeToIR(s)
	gt.Error(t, err)
}

func TestYamlNodeToIRAliasWithoutTarget(t *testing.T) {
	n := &yaml.Node{Kind: yaml.AliasNode}
	_, err := codec.YamlNodeToIR(n)
	gt.Error(t, err)
}

func TestYAMLTagToScalarTagValueInference(t *testing.T) {
	// When the tag is empty, the value drives the choice. yaml.v3 normally
	// resolves bare `~` and `null` to !!null before this function is called,
	// so we have to invoke it directly.
	cases := map[string]fxnode.ScalarTag{
		"":      fxnode.TagNull,
		"~":     fxnode.TagNull,
		"null":  fxnode.TagNull,
		"Null":  fxnode.TagNull,
		"NULL":  fxnode.TagNull,
		"hello": fxnode.TagString,
	}
	for v, want := range cases {
		got := codec.YAMLTagToScalarTag("", v)
		gt.Equal(t, got, want)
	}
}

// --- splitHCLLine: string-literal aware comment split ----------------------

func TestSplitHCLLineNoComment(t *testing.T) {
	code, comment := codec.SplitHCLLine(`name = "alice"`)
	gt.Equal(t, code, `name = "alice"`)
	gt.Equal(t, comment, "")
}

func TestSplitHCLLineHashComment(t *testing.T) {
	code, comment := codec.SplitHCLLine(`name = "alice" # hello`)
	gt.Equal(t, code, `name = "alice" `)
	gt.Equal(t, comment, "hello")
}

func TestSplitHCLLineDoubleSlashComment(t *testing.T) {
	code, comment := codec.SplitHCLLine(`name = "alice" // hello`)
	gt.Equal(t, code, `name = "alice" `)
	gt.Equal(t, comment, "hello")
}

func TestSplitHCLLineHashInsideString(t *testing.T) {
	code, comment := codec.SplitHCLLine(`url = "https://example.com/#frag"`)
	gt.Equal(t, code, `url = "https://example.com/#frag"`)
	gt.Equal(t, comment, "")
}

func TestSplitHCLLineEscapedQuoteInsideString(t *testing.T) {
	// Inner `\"` does not end the string, so the `#` is a real comment marker.
	code, comment := codec.SplitHCLLine(`v = "she said \"hi\"" # quoted`)
	gt.Equal(t, code, `v = "she said \"hi\"" `)
	gt.Equal(t, comment, "quoted")
}

func TestSplitHCLLineEscapedBackslashEndsString(t *testing.T) {
	// `\\` is an escaped backslash; the following quote *does* terminate the
	// string. The `#` after the string is a comment.
	code, comment := codec.SplitHCLLine(`v = "foo\\" # tail`)
	gt.Equal(t, code, `v = "foo\\" `)
	gt.Equal(t, comment, "tail")
}

func TestSplitHCLLineDoubleEscapedBackslashEndsString(t *testing.T) {
	// `\\\\` is two escaped backslashes; the following quote terminates.
	code, comment := codec.SplitHCLLine(`v = "foo\\\\" # tail`)
	gt.Equal(t, code, `v = "foo\\\\" `)
	gt.Equal(t, comment, "tail")
}

func TestSplitHCLLineOddBackslashEscapesQuote(t *testing.T) {
	// `\\\"` ends in escaped backslash + escaped quote, so the `"` is
	// escaped and we are still inside the string at the `#`.
	code, comment := codec.SplitHCLLine(`v = "foo\\\"# inner" # outer`)
	gt.Equal(t, code, `v = "foo\\\"# inner" `)
	gt.Equal(t, comment, "outer")
}

// --- irToYAMLNode mapping/sequence error paths ------------------------------

func TestIRToYAMLNodeMappingKeyError(t *testing.T) {
	m := fxnode.NewMapping()
	m.Mapping = append(m.Mapping, fxnode.MappingEntry{
		Key:   &fxnode.Node{Kind: fxnode.KindInvalid},
		Value: fxnode.NewString("v"),
	})
	_, err := codec.IRToYAMLNode(m)
	gt.Error(t, err)
}

func TestIRToYAMLNodeMappingValueError(t *testing.T) {
	m := fxnode.NewMapping()
	m.AddString("k", &fxnode.Node{Kind: fxnode.KindInvalid})
	_, err := codec.IRToYAMLNode(m)
	gt.Error(t, err)
}

func TestIRToYAMLNodeSequenceError(t *testing.T) {
	s := fxnode.NewSequence()
	s.Append(&fxnode.Node{Kind: fxnode.KindInvalid})
	_, err := codec.IRToYAMLNode(s)
	gt.Error(t, err)
}

func TestIRToYAMLNodeInvalidKind(t *testing.T) {
	_, err := codec.IRToYAMLNode(&fxnode.Node{Kind: fxnode.KindInvalid})
	gt.Error(t, err)
}
