package codec

import (
	"errors"
	"io"
	"strings"

	"github.com/m-mizutani/fx/internal/fxnode"
	"github.com/m-mizutani/goerr/v2"
	"gopkg.in/yaml.v3"
)

// yamlDecoder converts a YAML document into the IR. It uses yaml.Node so that
// comments and mapping key order are preserved.
type yamlDecoder struct{}

func (yamlDecoder) Decode(r io.Reader) (*fxnode.Node, error) {
	dec := yaml.NewDecoder(r)
	var doc yaml.Node
	if err := dec.Decode(&doc); err != nil {
		// Empty input is treated as an explicit null, matching YAML's
		// convention where a document containing only whitespace is null.
		if errors.Is(err, io.EOF) {
			return fxnode.NewNull(), nil
		}
		return nil, goerr.Wrap(err, "decode yaml")
	}
	// We only consume one document; concatenated documents (--- separated)
	// are rejected to keep semantics simple.
	var second yaml.Node
	if err := dec.Decode(&second); err == nil {
		return nil, goerr.New("multiple yaml documents are not supported")
	}

	n, err := yamlNodeToIR(&doc)
	if err != nil {
		return nil, goerr.Wrap(err, "translate yaml node")
	}
	return n, nil
}

// yamlNodeToIR translates a yaml.Node tree into the IR. DocumentNode unwraps
// to its single content child.
func yamlNodeToIR(y *yaml.Node) (*fxnode.Node, error) {
	switch y.Kind {
	case yaml.DocumentNode:
		if len(y.Content) == 0 {
			// Empty document — represent as null.
			return fxnode.NewNull(), nil
		}
		// Document-level comments wrap the first content node.
		n, err := yamlNodeToIR(y.Content[0])
		if err != nil {
			return nil, err
		}
		mergeComments(n, y)
		return n, nil

	case yaml.MappingNode:
		m := fxnode.NewMapping()
		applyComments(m, y)
		// Content alternates key, value, key, value, ...
		for i := 0; i+1 < len(y.Content); i += 2 {
			kn, err := yamlNodeToIR(y.Content[i])
			if err != nil {
				return nil, err
			}
			vn, err := yamlNodeToIR(y.Content[i+1])
			if err != nil {
				return nil, err
			}
			// yaml.v3 typically attaches the entry's HeadComment to the key
			// node and the LineComment to the value node. Lift the key's
			// HeadComment onto the value so encoders can place it correctly.
			if kn.HeadComment != "" && vn.HeadComment == "" {
				vn.HeadComment = kn.HeadComment
				kn.HeadComment = ""
			}
			m.Mapping = append(m.Mapping, fxnode.MappingEntry{Key: kn, Value: vn})
		}
		return m, nil

	case yaml.SequenceNode:
		s := fxnode.NewSequence()
		applyComments(s, y)
		for _, c := range y.Content {
			ch, err := yamlNodeToIR(c)
			if err != nil {
				return nil, err
			}
			s.Append(ch)
		}
		return s, nil

	case yaml.ScalarNode:
		tag := yamlTagToScalarTag(y.Tag, y.Value)
		// yaml.v3 emits an empty value with Tag !!null for explicit null and
		// for missing values; normalise both to TagNull.
		if tag == fxnode.TagNull {
			n := fxnode.NewNull()
			applyComments(n, y)
			return n, nil
		}
		n := fxnode.NewScalar(tag, y.Value)
		applyComments(n, y)
		return n, nil

	case yaml.AliasNode:
		// Resolve aliases by re-translating the target node. This loses the
		// anchor/alias relationship in output, which is an acceptable trade-off
		// for cross-format conversion.
		if y.Alias == nil {
			return nil, goerr.New("yaml alias without target")
		}
		return yamlNodeToIR(y.Alias)

	default:
		return nil, goerr.New("unsupported yaml node kind", goerr.V("kind", int(y.Kind)))
	}
}

// applyComments copies comment fields from a yaml.Node into the IR node.
func applyComments(n *fxnode.Node, y *yaml.Node) {
	n.HeadComment = stripCommentMarkers(y.HeadComment)
	n.LineComment = stripCommentMarkers(y.LineComment)
	n.FootComment = stripCommentMarkers(y.FootComment)
}

// mergeComments only fills in fields that are not already set, so that a
// child node's comments take precedence over outer wrappers.
func mergeComments(n *fxnode.Node, y *yaml.Node) {
	if n.HeadComment == "" {
		n.HeadComment = stripCommentMarkers(y.HeadComment)
	}
	if n.LineComment == "" {
		n.LineComment = stripCommentMarkers(y.LineComment)
	}
	if n.FootComment == "" {
		n.FootComment = stripCommentMarkers(y.FootComment)
	}
}

// stripCommentMarkers removes leading '#' and a single space from each line
// of a yaml comment block, leaving the raw text.
func stripCommentMarkers(s string) string {
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		trimmed := strings.TrimLeft(ln, " \t")
		if strings.HasPrefix(trimmed, "#") {
			trimmed = strings.TrimPrefix(trimmed, "#")
			trimmed = strings.TrimPrefix(trimmed, " ")
		}
		lines[i] = trimmed
	}
	return strings.Join(lines, "\n")
}

// yamlTagToScalarTag maps a YAML tag (or unset tag + value) to a ScalarTag.
// yaml.v3 leaves Tag empty for plain scalars and sets resolved tags after
// parsing; we mirror its resolution behaviour for the common subset.
func yamlTagToScalarTag(tag, value string) fxnode.ScalarTag {
	switch tag {
	case "!!str", "tag:yaml.org,2002:str":
		return fxnode.TagString
	case "!!int", "tag:yaml.org,2002:int":
		return fxnode.TagInt
	case "!!float", "tag:yaml.org,2002:float":
		return fxnode.TagFloat
	case "!!bool", "tag:yaml.org,2002:bool":
		return fxnode.TagBool
	case "!!null", "tag:yaml.org,2002:null":
		return fxnode.TagNull
	}
	// Fall back to value-based inference for plain scalars.
	switch value {
	case "", "~", "null", "Null", "NULL":
		return fxnode.TagNull
	}
	return fxnode.TagString
}

// yamlEncoder writes an IR tree as YAML, preserving comments.
type yamlEncoder struct{}

func (yamlEncoder) Encode(w io.Writer, n *fxnode.Node) error {
	y, err := irToYAMLNode(n)
	if err != nil {
		return goerr.Wrap(err, "translate ir to yaml node")
	}
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	defer func() { _ = enc.Close() }()
	if err := enc.Encode(y); err != nil {
		return goerr.Wrap(err, "encode yaml")
	}
	return nil
}

func irToYAMLNode(n *fxnode.Node) (*yaml.Node, error) {
	switch n.Kind {
	case fxnode.KindScalar:
		return irScalarToYAML(n), nil
	case fxnode.KindMapping:
		return irMappingToYAML(n)
	case fxnode.KindSequence:
		return irSequenceToYAML(n)
	default:
		return nil, goerr.New("invalid kind for yaml encode", goerr.V("kind", n.Kind.String()))
	}
}

func irScalarToYAML(n *fxnode.Node) *yaml.Node {
	y := &yaml.Node{Kind: yaml.ScalarNode}
	switch n.Scalar.Tag {
	case fxnode.TagString:
		y.Tag = "!!str"
		y.Value = n.Scalar.Repr
	case fxnode.TagInt:
		y.Tag = "!!int"
		y.Value = n.Scalar.Repr
	case fxnode.TagFloat:
		y.Tag = "!!float"
		y.Value = n.Scalar.Repr
	case fxnode.TagBool:
		y.Tag = "!!bool"
		y.Value = n.Scalar.Repr
	case fxnode.TagNull:
		y.Tag = "!!null"
		y.Value = "null"
	default:
		// Treat as plain string with no tag; yaml will resolve.
		y.Value = n.Scalar.Repr
	}
	applyIRComments(y, n)
	return y
}

func irMappingToYAML(n *fxnode.Node) (*yaml.Node, error) {
	y := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	applyIRComments(y, n)
	for _, e := range n.Mapping {
		k, err := irToYAMLNode(e.Key)
		if err != nil {
			return nil, err
		}
		v, err := irToYAMLNode(e.Value)
		if err != nil {
			return nil, err
		}
		// Entry-level HeadComment lives on the key node in yaml.v3.
		if v.HeadComment != "" && k.HeadComment == "" {
			k.HeadComment = v.HeadComment
			v.HeadComment = ""
		}
		y.Content = append(y.Content, k, v)
	}
	return y, nil
}

func irSequenceToYAML(n *fxnode.Node) (*yaml.Node, error) {
	y := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	applyIRComments(y, n)
	for _, c := range n.Sequence {
		ch, err := irToYAMLNode(c)
		if err != nil {
			return nil, err
		}
		y.Content = append(y.Content, ch)
	}
	return y, nil
}

func applyIRComments(y *yaml.Node, n *fxnode.Node) {
	y.HeadComment = n.HeadComment
	y.LineComment = n.LineComment
	y.FootComment = n.FootComment
}
