package fxnode_test

import (
	"testing"

	"github.com/m-mizutani/fx/internal/fxnode"
	"github.com/m-mizutani/gt"
)

func TestKindString(t *testing.T) {
	cases := map[fxnode.Kind]string{
		fxnode.KindScalar:   "scalar",
		fxnode.KindMapping:  "mapping",
		fxnode.KindSequence: "sequence",
		fxnode.KindInvalid:  "invalid",
		fxnode.Kind(99):     "invalid",
	}
	for k, want := range cases {
		gt.Equal(t, k.String(), want)
	}
}

func TestScalarTagString(t *testing.T) {
	cases := map[fxnode.ScalarTag]string{
		fxnode.TagString:     "string",
		fxnode.TagInt:        "int",
		fxnode.TagFloat:      "float",
		fxnode.TagBool:       "bool",
		fxnode.TagNull:       "null",
		fxnode.TagInvalid:    "invalid",
		fxnode.ScalarTag(99): "invalid",
	}
	for tag, want := range cases {
		gt.Equal(t, tag.String(), want)
	}
}

func TestNewScalarVariants(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		n := fxnode.NewString("hello")
		gt.Equal(t, n.Kind, fxnode.KindScalar)
		gt.Equal(t, n.Scalar.Tag, fxnode.TagString)
		gt.Equal(t, n.Scalar.Repr, "hello")
	})

	t.Run("int", func(t *testing.T) {
		n := fxnode.NewScalar(fxnode.TagInt, "42")
		gt.Equal(t, n.Scalar.Tag, fxnode.TagInt)
		gt.Equal(t, n.Scalar.Repr, "42")
	})

	t.Run("null", func(t *testing.T) {
		n := fxnode.NewNull()
		gt.Equal(t, n.Scalar.Tag, fxnode.TagNull)
		gt.Equal(t, n.Scalar.Repr, "")
	})
}

func TestMappingAddAndLookup(t *testing.T) {
	m := fxnode.NewMapping()
	gt.Equal(t, m.Kind, fxnode.KindMapping)
	gt.Equal(t, len(m.Mapping), 0)

	m.AddString("foo", fxnode.NewString("bar"))
	m.AddString("num", fxnode.NewScalar(fxnode.TagInt, "1"))

	got, ok := m.LookupString("foo")
	gt.True(t, ok)
	gt.Equal(t, got.Scalar.Repr, "bar")

	got, ok = m.LookupString("num")
	gt.True(t, ok)
	gt.Equal(t, got.Scalar.Tag, fxnode.TagInt)

	_, ok = m.LookupString("missing")
	gt.False(t, ok)
}

func TestLookupStringFirstWinsForDuplicates(t *testing.T) {
	m := fxnode.NewMapping()
	m.AddString("k", fxnode.NewString("first"))
	m.AddString("k", fxnode.NewString("second"))

	got, ok := m.LookupString("k")
	gt.True(t, ok)
	gt.Equal(t, got.Scalar.Repr, "first")
	gt.Equal(t, len(m.Mapping), 2) // both retained
}

func TestLookupStringOnNonMapping(t *testing.T) {
	s := fxnode.NewString("plain")
	_, ok := s.LookupString("anything")
	gt.False(t, ok)
}

func TestLookupStringSkipsNonScalarKey(t *testing.T) {
	m := fxnode.NewMapping()
	// A mapping with a non-scalar key should be ignored by LookupString.
	m.Add(fxnode.NewMapping(), fxnode.NewString("value"))
	m.AddString("real", fxnode.NewString("hit"))

	got, ok := m.LookupString("real")
	gt.True(t, ok)
	gt.Equal(t, got.Scalar.Repr, "hit")

	_, ok = m.LookupString("")
	gt.False(t, ok)
}

func TestLookupStringWithNilKey(t *testing.T) {
	m := fxnode.NewMapping()
	m.Mapping = append(m.Mapping, fxnode.MappingEntry{Key: nil, Value: fxnode.NewString("v")})
	_, ok := m.LookupString("")
	gt.False(t, ok)
}

func TestSequenceAppend(t *testing.T) {
	s := fxnode.NewSequence()
	gt.Equal(t, s.Kind, fxnode.KindSequence)

	s.Append(fxnode.NewString("a"))
	s.Append(fxnode.NewString("b"))

	gt.Equal(t, len(s.Sequence), 2)
	gt.Equal(t, s.Sequence[0].Scalar.Repr, "a")
	gt.Equal(t, s.Sequence[1].Scalar.Repr, "b")
}

func TestAddOnNonMappingPanics(t *testing.T) {
	s := fxnode.NewSequence()
	defer func() {
		r := recover()
		gt.NotNil(t, r)
	}()
	s.Add(fxnode.NewString("k"), fxnode.NewString("v"))
}

func TestAppendOnNonSequencePanics(t *testing.T) {
	m := fxnode.NewMapping()
	defer func() {
		r := recover()
		gt.NotNil(t, r)
	}()
	m.Append(fxnode.NewString("x"))
}

func TestComments(t *testing.T) {
	n := fxnode.NewString("v")
	n.HeadComment = "lead"
	n.LineComment = "trail"
	n.FootComment = "after"
	gt.Equal(t, n.HeadComment, "lead")
	gt.Equal(t, n.LineComment, "trail")
	gt.Equal(t, n.FootComment, "after")
}
