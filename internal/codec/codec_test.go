package codec_test

import (
	"testing"

	"github.com/m-mizutani/fx/internal/codec"
	"github.com/m-mizutani/fx/internal/format"
	"github.com/m-mizutani/gt"
)

func TestDecoderFor(t *testing.T) {
	for _, f := range format.All() {
		t.Run(string(f), func(t *testing.T) {
			d, err := codec.DecoderFor(f)
			gt.NoError(t, err)
			gt.NotNil(t, d)
		})
	}
}

func TestDecoderForUnknown(t *testing.T) {
	_, err := codec.DecoderFor(format.Format("xml"))
	gt.Error(t, err)
}

func TestEncoderFor(t *testing.T) {
	for _, f := range format.All() {
		t.Run(string(f), func(t *testing.T) {
			e, err := codec.EncoderFor(f)
			if f == format.FormatJsonnet {
				gt.Error(t, err)
				gt.Nil(t, e)
				return
			}
			gt.NoError(t, err)
			gt.NotNil(t, e)
		})
	}
}

func TestEncoderForUnknown(t *testing.T) {
	_, err := codec.EncoderFor(format.Format("xml"))
	gt.Error(t, err)
}
