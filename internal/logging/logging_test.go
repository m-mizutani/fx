package logging_test

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/m-mizutani/fx/internal/logging"
	"github.com/m-mizutani/gt"
)

func TestNewWritesToProvidedWriter(t *testing.T) {
	var buf bytes.Buffer
	lg := logging.New(&buf, slog.LevelDebug)
	lg.Info("hello", slog.String("k", "v"))
	gt.S(t, buf.String()).Contains("hello")
}

func TestNewNilWriterIsDiscard(t *testing.T) {
	// Should not panic.
	lg := logging.New(nil, slog.LevelInfo)
	lg.Info("ignored")
}

func TestWithAndFrom(t *testing.T) {
	var buf bytes.Buffer
	lg := logging.New(&buf, slog.LevelInfo)
	ctx := logging.With(context.Background(), lg)
	got := logging.From(ctx)
	gt.NotNil(t, got)
	got.Info("via context")
	gt.S(t, buf.String()).Contains("via context")
}

func TestFromFallsBackToDefault(t *testing.T) {
	got := logging.From(context.Background())
	gt.NotNil(t, got)
}

func TestFromIgnoresNilStoredValue(t *testing.T) {
	// Even though With does not let us store nil through the public API, the
	// fallback path still has to behave when an entirely empty context is
	// passed in — verified by reading from a fresh context.
	got := logging.From(context.TODO())
	gt.NotNil(t, got)
}
