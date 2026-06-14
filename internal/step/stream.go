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

// stdinKey is the context key for the per-step subprocess stdin reader.
type stdinKey struct{}

// ContextWithStdin returns a new context carrying a stdin reader. When
// RunCommand executes with this context, it attaches the reader as the
// subprocess's stdin so interactive prompts (e.g. `gh auth login`, git
// credential prompts) can read from the user. Passing a nil reader
// returns the parent context unchanged.
//
// Steps run serially within a single Runner, so only one subprocess at a
// time owns stdin — there is no risk of competing reads.
func ContextWithStdin(ctx context.Context, r io.Reader) context.Context {
	if r == nil {
		return ctx
	}
	return context.WithValue(ctx, stdinKey{}, r)
}

// StdinFromContext returns the reader set via ContextWithStdin, or nil if
// none is set. A nil result means the subprocess's stdin will be nil
// (the os/exec default), so any read from stdin returns EOF.
func StdinFromContext(ctx context.Context) io.Reader {
	r, _ := ctx.Value(stdinKey{}).(io.Reader)
	return r
}
