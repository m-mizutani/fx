package codec

import (
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/m-mizutani/fx/internal/fxnode"
	"github.com/m-mizutani/goerr/v2"
	"github.com/zclconf/go-cty/cty"
)

// hclBlockLabelsKey is the special mapping key used to carry HCL block labels
// in the IR. Other codecs treat this as a regular key; the HCL encoder also
// emits it as a plain attribute (we deliberately drop the block-vs-attribute
// distinction on round-trip to keep encoding simple).
const hclBlockLabelsKey = "_labels"

// hclDecoder parses an HCL file using hclsyntax (schema-less walk) and lowers
// it into the IR, lifting `#` / `//` comments onto the corresponding IR node
// as HeadComment / LineComment.
type hclDecoder struct{}

func (hclDecoder) Decode(r io.Reader) (*fxnode.Node, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, goerr.Wrap(err, "read hcl")
	}
	parser := hclparse.NewParser()
	file, diags := parser.ParseHCL(data, "input.hcl")
	if diags.HasErrors() {
		return nil, goerr.New("parse hcl", goerr.V("diagnostics", diags.Error()))
	}
	body, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return nil, goerr.New("unexpected hcl body type")
	}
	return decodeHCLBody(body, scanHCLComments(data))
}

// hclCommentMap is a line-indexed view of HCL source comments.
//   - head[L] is a head comment block whose attached code line is L
//     (the lines immediately above L that contain only `#` / `//` comments).
//   - line[L] is a trailing comment found on line L (after some code).
type hclCommentMap struct {
	head map[int]string
	line map[int]string
}

// scanHCLComments performs a string-literal-aware line scan and pairs
// stand-alone comment blocks with the next code line, and trailing comments
// with their own line. Block comments (`/* … */`) are not interpreted.
func scanHCLComments(src []byte) *hclCommentMap {
	cm := &hclCommentMap{head: map[int]string{}, line: map[int]string{}}
	lines := strings.Split(string(src), "\n")
	var buf []string
	for i, ln := range lines {
		lineNo := i + 1
		code, comment := splitHCLLine(ln)
		codeTrim := strings.TrimSpace(code)
		switch {
		case codeTrim == "" && comment != "":
			buf = append(buf, comment)
		case codeTrim == "" && comment == "":
			// Blank line clears any accumulating head block.
			buf = nil
		default:
			if len(buf) > 0 {
				cm.head[lineNo] = strings.Join(buf, "\n")
				buf = nil
			}
			if comment != "" {
				cm.line[lineNo] = comment
			}
		}
	}
	return cm
}

// splitHCLLine returns (code, comment) for a single HCL source line.
// String literals are masked so `#` inside a value does not become a
// comment marker.
func splitHCLLine(line string) (code, comment string) {
	masked := []byte(line)
	inStr := false
	for i := 0; i < len(masked); i++ {
		c := masked[i]
		if c == '"' && (i == 0 || masked[i-1] != '\\') {
			inStr = !inStr
			continue
		}
		if inStr {
			masked[i] = ' '
		}
	}
	s := string(masked)
	pos := -1
	skip := 1
	for i := 0; i < len(s); i++ {
		if s[i] == '#' {
			pos = i
			skip = 1
			break
		}
		if i+1 < len(s) && s[i] == '/' && s[i+1] == '/' {
			pos = i
			skip = 2
			break
		}
	}
	if pos < 0 {
		return line, ""
	}
	return line[:pos], strings.TrimSpace(line[pos+skip:])
}

// decodeHCLBody walks a hclsyntax.Body into an IR mapping, lifting comments
// from the source into the relevant IR nodes via the comment map.
func decodeHCLBody(body *hclsyntax.Body, cm *hclCommentMap) (*fxnode.Node, error) {
	m := fxnode.NewMapping()

	// Attributes — sort by name for deterministic output.
	attrNames := make([]string, 0, len(body.Attributes))
	for name := range body.Attributes {
		attrNames = append(attrNames, name)
	}
	sort.Strings(attrNames)
	for _, name := range attrNames {
		attr := body.Attributes[name]
		val, diags := attr.Expr.Value(nil)
		if diags.HasErrors() {
			return nil, goerr.New("evaluate hcl attribute",
				goerr.V("name", name), goerr.V("diagnostics", diags.Error()))
		}
		n, err := ctyToIR(val)
		if err != nil {
			return nil, goerr.Wrap(err, "translate cty", goerr.V("attr", name))
		}
		if c, ok := cm.head[attr.NameRange.Start.Line]; ok {
			n.HeadComment = c
		}
		if c, ok := cm.line[attr.SrcRange.End.Line]; ok {
			n.LineComment = c
		}
		m.AddString(name, n)
	}

	// Blocks — group by Type, preserving order of first appearance.
	type group struct {
		Type   string
		Blocks []*hclsyntax.Block
	}
	var groups []group
	idx := map[string]int{}
	for _, blk := range body.Blocks {
		if i, ok := idx[blk.Type]; ok {
			groups[i].Blocks = append(groups[i].Blocks, blk)
			continue
		}
		idx[blk.Type] = len(groups)
		groups = append(groups, group{Type: blk.Type, Blocks: []*hclsyntax.Block{blk}})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Type < groups[j].Type })

	for _, g := range groups {
		nodes := make([]*fxnode.Node, 0, len(g.Blocks))
		for _, blk := range g.Blocks {
			inner, err := decodeHCLBody(blk.Body, cm)
			if err != nil {
				return nil, err
			}
			if len(blk.Labels) > 0 {
				labelsSeq := fxnode.NewSequence()
				for _, l := range blk.Labels {
					labelsSeq.Append(fxnode.NewString(l))
				}
				inner.Mapping = append(
					[]fxnode.MappingEntry{{Key: fxnode.NewString(hclBlockLabelsKey), Value: labelsSeq}},
					inner.Mapping...,
				)
			}
			// Block-level head/line comments are attached to the resulting
			// mapping node so a future encoder can re-emit them.
			if c, ok := cm.head[blk.TypeRange.Start.Line]; ok {
				inner.HeadComment = c
			}
			if c, ok := cm.line[blk.OpenBraceRange.Start.Line]; ok {
				inner.LineComment = c
			}
			nodes = append(nodes, inner)
		}
		if len(nodes) == 1 {
			m.AddString(g.Type, nodes[0])
		} else {
			seq := fxnode.NewSequence()
			for _, n := range nodes {
				seq.Append(n)
			}
			m.AddString(g.Type, seq)
		}
	}

	return m, nil
}

// ctyToIR converts a cty.Value to an IR node. Mapping ordering for object/map
// types is alphabetical so that the encoder output is stable.
func ctyToIR(v cty.Value) (*fxnode.Node, error) {
	if !v.IsKnown() {
		return nil, goerr.New("hcl value is not known (depends on variables?)")
	}
	if v.IsNull() {
		return fxnode.NewNull(), nil
	}
	ty := v.Type()
	switch {
	case ty == cty.String:
		return fxnode.NewString(v.AsString()), nil
	case ty == cty.Number:
		bf := v.AsBigFloat()
		if bf.IsInt() {
			i, _ := bf.Int64()
			return fxnode.NewScalar(fxnode.TagInt, strconv.FormatInt(i, 10)), nil
		}
		f, _ := bf.Float64()
		return fxnode.NewScalar(fxnode.TagFloat, strconv.FormatFloat(f, 'g', -1, 64)), nil
	case ty == cty.Bool:
		if v.True() {
			return fxnode.NewScalar(fxnode.TagBool, "true"), nil
		}
		return fxnode.NewScalar(fxnode.TagBool, "false"), nil
	case ty.IsTupleType() || ty.IsListType() || ty.IsSetType():
		s := fxnode.NewSequence()
		it := v.ElementIterator()
		for it.Next() {
			_, ev := it.Element()
			n, err := ctyToIR(ev)
			if err != nil {
				return nil, err
			}
			s.Append(n)
		}
		return s, nil
	case ty.IsObjectType():
		attrs := ty.AttributeTypes()
		keys := make([]string, 0, len(attrs))
		for k := range attrs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		m := fxnode.NewMapping()
		for _, k := range keys {
			n, err := ctyToIR(v.GetAttr(k))
			if err != nil {
				return nil, err
			}
			m.AddString(k, n)
		}
		return m, nil
	case ty.IsMapType():
		type kv struct {
			k string
			v cty.Value
		}
		var entries []kv
		it := v.ElementIterator()
		for it.Next() {
			k, vv := it.Element()
			entries = append(entries, kv{k.AsString(), vv})
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].k < entries[j].k })
		m := fxnode.NewMapping()
		for _, e := range entries {
			n, err := ctyToIR(e.v)
			if err != nil {
				return nil, err
			}
			m.AddString(e.k, n)
		}
		return m, nil
	default:
		return nil, goerr.New("unsupported cty type", goerr.V("type", ty.FriendlyName()))
	}
}

// hclEncoder writes the IR as HCL. It builds attributes via raw tokens so we
// can interleave HeadComment / LineComment around each attribute. Mapping
// values are emitted as object literals (no block syntax).
type hclEncoder struct{}

func (hclEncoder) Encode(w io.Writer, n *fxnode.Node) error {
	if n.Kind != fxnode.KindMapping {
		return goerr.New("hcl top level must be a mapping",
			goerr.V("kind", n.Kind.String()))
	}
	f := hclwrite.NewEmptyFile()
	body := f.Body()
	for _, e := range n.Mapping {
		if e.Key == nil || e.Key.Kind != fxnode.KindScalar {
			return goerr.New("hcl mapping key must be a string scalar")
		}
		if err := writeHCLAttribute(body, e.Key.Scalar.Repr, e.Value); err != nil {
			return goerr.Wrap(err, "write hcl attribute", goerr.V("key", e.Key.Scalar.Repr))
		}
	}
	if _, err := w.Write(f.Bytes()); err != nil {
		return goerr.Wrap(err, "write hcl")
	}
	return nil
}

// writeHCLAttribute renders a single `name = value` attribute, optionally
// preceded by a head comment block and trailed by a line comment.
func writeHCLAttribute(body *hclwrite.Body, name string, v *fxnode.Node) error {
	if v != nil && v.HeadComment != "" {
		body.AppendUnstructuredTokens(hclHeadCommentTokens(v.HeadComment))
	}
	ctyVal, err := irToCty(v)
	if err != nil {
		return err
	}
	// Build `name = value [# line-comment]\n` as raw tokens, so the line
	// comment lands before the newline (which is what users expect).
	toks := hclwrite.Tokens{
		{Type: hclsyntax.TokenIdent, Bytes: []byte(name)},
		{Type: hclsyntax.TokenEqual, Bytes: []byte("="), SpacesBefore: 1},
	}
	valToks := hclwrite.TokensForValue(ctyVal)
	if len(valToks) > 0 {
		valToks[0].SpacesBefore = 1
	}
	toks = append(toks, valToks...)
	if v != nil && v.LineComment != "" {
		toks = append(toks, &hclwrite.Token{
			Type:         hclsyntax.TokenComment,
			Bytes:        []byte("# " + v.LineComment),
			SpacesBefore: 1,
		})
	}
	toks = append(toks, &hclwrite.Token{
		Type:  hclsyntax.TokenNewline,
		Bytes: []byte("\n"),
	})
	body.AppendUnstructuredTokens(toks)
	return nil
}

// hclHeadCommentTokens renders a multi-line head comment as a sequence of
// `# line\n` tokens.
func hclHeadCommentTokens(s string) hclwrite.Tokens {
	var toks hclwrite.Tokens
	for _, ln := range strings.Split(s, "\n") {
		toks = append(toks, &hclwrite.Token{
			Type:  hclsyntax.TokenComment,
			Bytes: []byte("# " + ln + "\n"),
		})
	}
	return toks
}

// irToCty converts an IR node to a cty.Value.
func irToCty(n *fxnode.Node) (cty.Value, error) {
	switch n.Kind {
	case fxnode.KindScalar:
		return scalarToCty(n.Scalar)
	case fxnode.KindSequence:
		if len(n.Sequence) == 0 {
			return cty.EmptyTupleVal, nil
		}
		vals := make([]cty.Value, 0, len(n.Sequence))
		for _, c := range n.Sequence {
			v, err := irToCty(c)
			if err != nil {
				return cty.NilVal, err
			}
			vals = append(vals, v)
		}
		return cty.TupleVal(vals), nil
	case fxnode.KindMapping:
		if len(n.Mapping) == 0 {
			return cty.EmptyObjectVal, nil
		}
		attrs := make(map[string]cty.Value, len(n.Mapping))
		for _, e := range n.Mapping {
			if e.Key == nil || e.Key.Kind != fxnode.KindScalar {
				return cty.NilVal, goerr.New("non-string key in hcl mapping")
			}
			v, err := irToCty(e.Value)
			if err != nil {
				return cty.NilVal, err
			}
			attrs[e.Key.Scalar.Repr] = v
		}
		return cty.ObjectVal(attrs), nil
	default:
		return cty.NilVal, goerr.New("invalid kind", goerr.V("kind", n.Kind.String()))
	}
}

func scalarToCty(s fxnode.ScalarValue) (cty.Value, error) {
	switch s.Tag {
	case fxnode.TagString:
		return cty.StringVal(s.Repr), nil
	case fxnode.TagInt:
		i, err := strconv.ParseInt(s.Repr, 10, 64)
		if err != nil {
			return cty.NilVal, goerr.Wrap(err, "parse int", goerr.V("repr", s.Repr))
		}
		return cty.NumberIntVal(i), nil
	case fxnode.TagFloat:
		f, err := strconv.ParseFloat(s.Repr, 64)
		if err != nil {
			return cty.NilVal, goerr.Wrap(err, "parse float", goerr.V("repr", s.Repr))
		}
		return cty.NumberFloatVal(f), nil
	case fxnode.TagBool:
		return cty.BoolVal(s.Repr == "true"), nil
	case fxnode.TagNull:
		return cty.NullVal(cty.DynamicPseudoType), nil
	default:
		return cty.NilVal, goerr.New("invalid scalar tag", goerr.V("tag", s.Tag.String()))
	}
}

// _ uses an HCL diagnostic helper to keep the import surface stable across
// hcl versions; this avoids "imported and not used" if some symbols change.
var _ = hcl.NewDiagnosticTextWriter
