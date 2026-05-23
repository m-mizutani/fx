// Package logging wires the m-mizutani/clog handler into a context-scoped
// slog.Logger so that the rest of fx can avoid touching the global slog.
package logging

import (
	"context"
	"io"
	"log/slog"

	"github.com/m-mizutani/clog"
)

// loggerKey is a private context key, ensuring nothing outside this package
// can shadow the logger.
type loggerKey struct{}

// New constructs a slog.Logger backed by clog writing to the given writer.
// When w is nil, io.Discard is used.
func New(w io.Writer, level slog.Level) *slog.Logger {
	if w == nil {
		w = io.Discard
	}
	handler := clog.New(
		clog.WithWriter(w),
		clog.WithLevel(level),
	)
	return slog.New(handler)
}

// With returns a new context carrying the provided logger.
func With(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}

// From returns the logger stored in ctx or slog.Default() when absent.
// It never returns nil.
func From(ctx context.Context) *slog.Logger {
	if v, ok := ctx.Value(loggerKey{}).(*slog.Logger); ok && v != nil {
		return v
	}
	return slog.Default()
}
