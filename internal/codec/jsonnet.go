package codec

import (
	"io"
	"strings"

	"github.com/google/go-jsonnet"
	"github.com/m-mizutani/fx/internal/fxnode"
	"github.com/m-mizutani/goerr/v2"
)

// jsonnetDecoder evaluates a Jsonnet snippet and decodes the resulting JSON
// into the IR. Comments are not preserved (the evaluation collapses the AST
// into a plain JSON value); see spec for the rationale.
type jsonnetDecoder struct{}

func (jsonnetDecoder) Decode(r io.Reader) (*fxnode.Node, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, goerr.Wrap(err, "read jsonnet")
	}
	vm := jsonnet.MakeVM()
	out, err := vm.EvaluateAnonymousSnippet("input.jsonnet", string(data))
	if err != nil {
		return nil, goerr.Wrap(err, "evaluate jsonnet")
	}
	n, err := jsonDecoder{}.Decode(strings.NewReader(out))
	if err != nil {
		return nil, goerr.Wrap(err, "decode jsonnet output as json")
	}
	return n, nil
}
