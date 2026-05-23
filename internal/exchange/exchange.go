// Package exchange is the high-level convert function used by the CLI. It
// pairs a Decoder with an Encoder selected from the codec registry and
// streams data from one to the other through the IR.
package exchange

import (
	"io"

	"github.com/m-mizutani/fx/internal/codec"
	"github.com/m-mizutani/fx/internal/format"
	"github.com/m-mizutani/goerr/v2"
)

// Exchange reads from r in src format and writes to w in dst format.
// Errors from either side are wrapped so the caller can attribute failures.
func Exchange(src, dst format.Format, r io.Reader, w io.Writer) error {
	decoder, err := codec.DecoderFor(src)
	if err != nil {
		return goerr.Wrap(err, "select decoder", goerr.V("src", string(src)))
	}
	encoder, err := codec.EncoderFor(dst)
	if err != nil {
		return goerr.Wrap(err, "select encoder", goerr.V("dst", string(dst)))
	}
	node, err := decoder.Decode(r)
	if err != nil {
		return goerr.Wrap(err, "decode", goerr.V("src", string(src)))
	}
	if err := encoder.Encode(w, node); err != nil {
		return goerr.Wrap(err, "encode", goerr.V("dst", string(dst)))
	}
	return nil
}
