package exchange_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/m-mizutani/fx/internal/exchange"
	"github.com/m-mizutani/fx/internal/format"
	"github.com/m-mizutani/gt"
)

// representativeInputs holds a minimal valid document for each format that
// can be decoded as input. They are designed so the resulting IR is
// expressible in every output format (mapping at top level, primitive
// scalars only).
var representativeInputs = map[format.Format]string{
	format.FormatJSON: `{"name": "alice", "age": 30, "active": true}`,
	format.FormatYAML: "name: alice\nage: 30\nactive: true\n",
	format.FormatTOML: "name = \"alice\"\nage = 30\nactive = true\n",
	format.FormatHCL:  "name = \"alice\"\nage = 30\nactive = true\n",
	format.FormatJsonnet: `{
  name: "alice",
  age: 30,
  active: true,
}`,
}

func TestExchangeAllPairs(t *testing.T) {
	dsts := []format.Format{format.FormatJSON, format.FormatYAML, format.FormatTOML, format.FormatHCL}
	for src, in := range representativeInputs {
		for _, dst := range dsts {
			t.Run(string(src)+"_to_"+string(dst), func(t *testing.T) {
				var buf bytes.Buffer
				err := exchange.Exchange(src, dst, strings.NewReader(in), &buf)
				gt.NoError(t, err)
				gt.S(t, buf.String()).Contains("alice")
			})
		}
	}
}

func TestExchangeToJsonnetIsError(t *testing.T) {
	err := exchange.Exchange(format.FormatJSON, format.FormatJsonnet, strings.NewReader(`{}`), &bytes.Buffer{})
	gt.Error(t, err)
}

func TestExchangeUnknownSrc(t *testing.T) {
	err := exchange.Exchange(format.Format("xml"), format.FormatJSON, strings.NewReader(`{}`), &bytes.Buffer{})
	gt.Error(t, err)
}

func TestExchangeUnknownDst(t *testing.T) {
	err := exchange.Exchange(format.FormatJSON, format.Format("xml"), strings.NewReader(`{}`), &bytes.Buffer{})
	gt.Error(t, err)
}

func TestExchangeDecodeError(t *testing.T) {
	err := exchange.Exchange(format.FormatJSON, format.FormatYAML, strings.NewReader(`{invalid`), &bytes.Buffer{})
	gt.Error(t, err)
}

func TestExchangeEncodeError(t *testing.T) {
	// JSON array → TOML errors because TOML requires a mapping at the top.
	err := exchange.Exchange(format.FormatJSON, format.FormatTOML, strings.NewReader(`[1,2,3]`), &bytes.Buffer{})
	gt.Error(t, err)
}
