// Package codec defines the Decoder/Encoder interfaces used by fx and
// provides one implementation per supported format. The registry exposed by
// DecoderFor / EncoderFor wires format.Format values to concrete codecs.
package codec

import (
	"io"

	"github.com/m-mizutani/fx/internal/format"
	"github.com/m-mizutani/fx/internal/fxnode"
	"github.com/m-mizutani/goerr/v2"
)

// Decoder reads a stream of a particular format and produces an IR tree.
type Decoder interface {
	Decode(r io.Reader) (*fxnode.Node, error)
}

// Encoder writes an IR tree to the given writer in a particular format.
type Encoder interface {
	Encode(w io.Writer, n *fxnode.Node) error
}

// DecoderFor returns the Decoder registered for f. Returns an error for
// formats that do not support decoding (none currently — all formats decode).
func DecoderFor(f format.Format) (Decoder, error) {
	switch f {
	case format.FormatJSON:
		return jsonDecoder{}, nil
	case format.FormatYAML:
		return yamlDecoder{}, nil
	case format.FormatTOML:
		return tomlDecoder{}, nil
	case format.FormatHCL:
		return hclDecoder{}, nil
	case format.FormatJsonnet:
		return jsonnetDecoder{}, nil
	default:
		return nil, goerr.New("no decoder for format", goerr.V("format", string(f)))
	}
}

// EncoderFor returns the Encoder registered for f. Jsonnet is a decode-only
// format and returns an error here, matching the documented behaviour.
func EncoderFor(f format.Format) (Encoder, error) {
	switch f {
	case format.FormatJSON:
		return jsonEncoder{}, nil
	case format.FormatYAML:
		return yamlEncoder{}, nil
	case format.FormatTOML:
		return tomlEncoder{}, nil
	case format.FormatHCL:
		return hclEncoder{}, nil
	case format.FormatJsonnet:
		return nil, goerr.New("jsonnet is decode-only; cannot encode to jsonnet",
			goerr.V("format", string(f)))
	default:
		return nil, goerr.New("no encoder for format", goerr.V("format", string(f)))
	}
}
