// Package fxnode provides the format-independent intermediate representation
// (IR) used by fx to round-trip structured data between supported formats.
//
// The IR is intentionally lossless about ordering and comments: mappings keep
// insertion order, and each node may carry head / line / foot comments so that
// codecs which support comment metadata can preserve them across formats.
package fxnode

// Kind classifies a Node's payload.
type Kind int

const (
	// KindInvalid is the zero value and indicates an uninitialised node.
	KindInvalid Kind = iota
	// KindScalar represents a leaf value (string / int / float / bool / null).
	KindScalar
	// KindMapping represents an ordered set of key-value pairs.
	KindMapping
	// KindSequence represents an ordered list of nodes.
	KindSequence
)

// String returns a human-readable name for the Kind. Useful for error messages.
func (k Kind) String() string {
	switch k {
	case KindScalar:
		return "scalar"
	case KindMapping:
		return "mapping"
	case KindSequence:
		return "sequence"
	default:
		return "invalid"
	}
}

// ScalarTag tags a ScalarValue with its semantic type so encoders can render
// the value with the correct format-specific syntax.
type ScalarTag int

const (
	// TagInvalid is the zero value.
	TagInvalid ScalarTag = iota
	// TagString is a quoted/unquoted string scalar.
	TagString
	// TagInt is an integer scalar.
	TagInt
	// TagFloat is a floating-point scalar.
	TagFloat
	// TagBool is a boolean scalar.
	TagBool
	// TagNull is an explicit null / nil value.
	TagNull
)

// String returns a human-readable name for the ScalarTag.
func (t ScalarTag) String() string {
	switch t {
	case TagString:
		return "string"
	case TagInt:
		return "int"
	case TagFloat:
		return "float"
	case TagBool:
		return "bool"
	case TagNull:
		return "null"
	default:
		return "invalid"
	}
}

// ScalarValue holds a scalar's canonical textual form plus a semantic tag.
// Repr is the textual representation as emitted by the source decoder
// (e.g. "42", "3.14", "true"). For TagNull, Repr is conventionally "".
type ScalarValue struct {
	Repr string
	Tag  ScalarTag
}

// MappingEntry is one key-value pair in a KindMapping Node.
// Keys are typically KindScalar nodes; codecs that allow non-string keys
// (e.g. YAML) may use other kinds, but most output formats require strings.
type MappingEntry struct {
	Key   *Node
	Value *Node
}

// Node is the universal IR node. Exactly one of Scalar / Mapping / Sequence
// is meaningful depending on Kind; the other fields are zero-valued.
//
// Comment fields hold the raw text without the source format's comment markers
// (e.g. no leading "#" for YAML, no "//" for HCL). Multi-line comments are
// joined with "\n".
type Node struct {
	Kind Kind

	Scalar   ScalarValue
	Mapping  []MappingEntry
	Sequence []*Node

	HeadComment string
	LineComment string
	FootComment string
}

// NewScalar constructs a scalar Node.
func NewScalar(tag ScalarTag, repr string) *Node {
	return &Node{Kind: KindScalar, Scalar: ScalarValue{Repr: repr, Tag: tag}}
}

// NewString is a convenience constructor for string scalars.
func NewString(s string) *Node { return NewScalar(TagString, s) }

// NewMapping constructs an empty mapping Node.
func NewMapping() *Node { return &Node{Kind: KindMapping} }

// NewSequence constructs an empty sequence Node.
func NewSequence() *Node { return &Node{Kind: KindSequence} }

// NewNull constructs an explicit null scalar.
func NewNull() *Node { return NewScalar(TagNull, "") }

// Add appends an entry to a mapping node.
// Panics if called on a non-mapping node, which would indicate a programmer
// error in a codec implementation.
func (n *Node) Add(key, value *Node) {
	if n.Kind != KindMapping {
		panic("fxnode: Add called on non-mapping node (kind=" + n.Kind.String() + ")")
	}
	n.Mapping = append(n.Mapping, MappingEntry{Key: key, Value: value})
}

// AddString is shorthand for adding a string-keyed entry.
func (n *Node) AddString(key string, value *Node) {
	n.Add(NewString(key), value)
}

// Append appends a child node to a sequence.
// Panics if called on a non-sequence node.
func (n *Node) Append(v *Node) {
	if n.Kind != KindSequence {
		panic("fxnode: Append called on non-sequence node (kind=" + n.Kind.String() + ")")
	}
	n.Sequence = append(n.Sequence, v)
}

// LookupString returns the first value whose key is a string scalar equal to
// the given key. Returns (nil, false) when not found, or when the receiver is
// not a mapping. First match wins; duplicate keys are preserved in Mapping
// but only the first is exposed via this helper.
func (n *Node) LookupString(key string) (*Node, bool) {
	if n.Kind != KindMapping {
		return nil, false
	}
	for _, e := range n.Mapping {
		if e.Key != nil && e.Key.Kind == KindScalar && e.Key.Scalar.Repr == key {
			return e.Value, true
		}
	}
	return nil, false
}
