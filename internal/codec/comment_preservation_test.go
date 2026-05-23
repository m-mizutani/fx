package codec_test

// comment_preservation_test.go exercises the end-to-end comment-handling
// contract from the spec: HeadComment / LineComment must survive a decode
// + encode round-trip in every format that has comment grammar, and must
// also travel across formats (yaml ↔ hcl in particular).

import (
	"bytes"
	"strings"
	"testing"

	"github.com/m-mizutani/fx/internal/codec"
	"github.com/m-mizutani/fx/internal/fxnode"
	"github.com/m-mizutani/gt"
)

// --- HCL self round-trip ----------------------------------------------------

func TestHCLDecodeHeadComment(t *testing.T) {
	dec, _ := codec.DecoderFor("hcl")
	data := readTestdata(t, "hcl/comment_head.hcl")
	n, err := dec.Decode(bytes.NewReader(data))
	gt.NoError(t, err)

	name, ok := n.LookupString("name")
	gt.True(t, ok)
	gt.S(t, name.HeadComment).Contains("Server configuration")
	gt.S(t, name.HeadComment).Contains("multi-line head comment")
}

func TestHCLDecodeLineComment(t *testing.T) {
	dec, _ := codec.DecoderFor("hcl")
	data := readTestdata(t, "hcl/comment_line.hcl")
	n, err := dec.Decode(bytes.NewReader(data))
	gt.NoError(t, err)

	name, _ := n.LookupString("name")
	gt.Equal(t, name.LineComment, "the primary user")

	port, _ := n.LookupString("port")
	gt.Equal(t, port.LineComment, "numeric port")
}

func TestHCLRoundTripPreservesComments(t *testing.T) {
	dec, _ := codec.DecoderFor("hcl")
	enc, _ := codec.EncoderFor("hcl")

	data := readTestdata(t, "hcl/comment_mixed.hcl")
	n, err := dec.Decode(bytes.NewReader(data))
	gt.NoError(t, err)

	var buf bytes.Buffer
	gt.NoError(t, enc.Encode(&buf, n))
	out := buf.String()

	// Head comments come back as `# …` lines.
	gt.S(t, out).Contains("Application metadata")
	gt.S(t, out).Contains("Server settings")
	// Line comments stay on their attribute line.
	gt.S(t, out).Contains("the product name")
	gt.S(t, out).Contains("default port")

	// Verify the line comment sits on the same line as the attribute it
	// belongs to (not pushed to the following line).
	for _, want := range []string{"the product name", "default port"} {
		ok := false
		for _, ln := range strings.Split(out, "\n") {
			if strings.Contains(ln, want) {
				// The line should also contain the attribute name or value.
				if strings.Contains(ln, "=") {
					ok = true
					break
				}
			}
		}
		if !ok {
			t.Errorf("line comment %q is not on the attribute line:\n%s", want, out)
		}
	}
}

// --- YAML → HCL -------------------------------------------------------------

func TestYAMLToHCLPreservesHeadComment(t *testing.T) {
	yamlDec, _ := codec.DecoderFor("yaml")
	hclEnc, _ := codec.EncoderFor("hcl")

	src := readTestdata(t, "yaml/comment_head.yaml")
	n, err := yamlDec.Decode(bytes.NewReader(src))
	gt.NoError(t, err)

	var buf bytes.Buffer
	gt.NoError(t, hclEnc.Encode(&buf, n))
	out := buf.String()

	// The yaml file contains: "# Server configuration block" above `server:`
	// and "# The port to listen on" above `port:` (which is nested under
	// `server`). HCL output renders the mapping as an attribute with an
	// object literal — head comments on top-level entries survive.
	gt.S(t, out).Contains("Server configuration block")
}

func TestYAMLToHCLPreservesLineComment(t *testing.T) {
	yamlDec, _ := codec.DecoderFor("yaml")
	hclEnc, _ := codec.EncoderFor("hcl")

	src := readTestdata(t, "yaml/comment_line.yaml")
	n, err := yamlDec.Decode(bytes.NewReader(src))
	gt.NoError(t, err)

	var buf bytes.Buffer
	gt.NoError(t, hclEnc.Encode(&buf, n))
	out := buf.String()

	// Line comments live on the same line as the attribute they decorate.
	gt.S(t, out).Contains("the primary user")
	gt.S(t, out).Contains("in years")
}

// --- HCL → YAML -------------------------------------------------------------

func TestHCLToYAMLPreservesHeadComment(t *testing.T) {
	hclDec, _ := codec.DecoderFor("hcl")
	yamlEnc, _ := codec.EncoderFor("yaml")

	src := readTestdata(t, "hcl/comment_head.hcl")
	n, err := hclDec.Decode(bytes.NewReader(src))
	gt.NoError(t, err)

	var buf bytes.Buffer
	gt.NoError(t, yamlEnc.Encode(&buf, n))
	out := buf.String()

	gt.S(t, out).Contains("Server configuration")
}

func TestHCLToYAMLPreservesLineComment(t *testing.T) {
	hclDec, _ := codec.DecoderFor("hcl")
	yamlEnc, _ := codec.EncoderFor("yaml")

	src := readTestdata(t, "hcl/comment_line.hcl")
	n, err := hclDec.Decode(bytes.NewReader(src))
	gt.NoError(t, err)

	var buf bytes.Buffer
	gt.NoError(t, yamlEnc.Encode(&buf, n))
	out := buf.String()

	gt.S(t, out).Contains("the primary user")
	gt.S(t, out).Contains("numeric port")
}

// --- JSON drops comments silently ------------------------------------------

func TestHCLToJSONDropsComments(t *testing.T) {
	hclDec, _ := codec.DecoderFor("hcl")
	jsonEnc, _ := codec.EncoderFor("json")

	src := readTestdata(t, "hcl/comment_head.hcl")
	n, err := hclDec.Decode(bytes.NewReader(src))
	gt.NoError(t, err)

	var buf bytes.Buffer
	gt.NoError(t, jsonEnc.Encode(&buf, n))
	out := buf.String()

	// JSON has no comment grammar — the original comment text must be gone.
	gt.False(t, strings.Contains(out, "Server configuration"))
	// But the data is intact.
	gt.S(t, out).Contains(`"alice"`)
}

// --- Programmatic IR → HCL --------------------------------------------------

func TestHCLEncodeIRWithComments(t *testing.T) {
	enc, _ := codec.EncoderFor("hcl")
	m := fxnode.NewMapping()

	name := fxnode.NewString("alice")
	name.HeadComment = "the user record"
	name.LineComment = "primary"
	m.AddString("name", name)

	age := fxnode.NewScalar(fxnode.TagInt, "30")
	age.HeadComment = "age in years\n(integer)"
	m.AddString("age", age)

	var buf bytes.Buffer
	gt.NoError(t, enc.Encode(&buf, m))
	out := buf.String()

	gt.S(t, out).Contains("# the user record")
	gt.S(t, out).Contains("# primary")
	gt.S(t, out).Contains("# age in years")
	gt.S(t, out).Contains("# (integer)")
}
