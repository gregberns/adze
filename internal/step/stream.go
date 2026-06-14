package step

import (
	"context"
	"io"
)

// streamWriterKey is the context key for the per-step subprocess stream writer.
type streamWriterKey struct{}

// ContextWithStreamWriter returns a new context carrying a stream writer.
// When RunCommand executes with this context, it tees subprocess stdout
// and stderr to the writer in addition to capturing into ExecResult.
// Passing a nil writer returns the parent context unchanged.
func ContextWithStreamWriter(ctx context.Context, w io.Writer) context.Context {
	if w == nil {
		return ctx
	}
	return context.WithValue(ctx, streamWriterKey{}, w)
}

// StreamWriterFromContext returns the writer set via ContextWithStreamWriter,
// or nil if no writer is set.
func StreamWriterFromContext(ctx context.Context) io.Writer {
	w, _ := ctx.Value(streamWriterKey{}).(io.Writer)
	return w
}
