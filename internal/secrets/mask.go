package secrets

import (
	"io"
	"strings"
	"sync"
)

// MaskingFilter replaces sensitive values in output with ***.
type MaskingFilter struct {
	mu     sync.RWMutex
	values []string
}

// NewMaskingFilter creates a new masking filter.
func NewMaskingFilter() *MaskingFilter {
	return &MaskingFilter{}
}

// Register adds a sensitive value to be masked.
// Empty strings are ignored.
func (f *MaskingFilter) Register(value string) {
	if value == "" {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.values = append(f.values, value)
}

// Mask replaces all registered values in the input with ***.
func (f *MaskingFilter) Mask(input string) string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	result := input
	for _, v := range f.values {
		result = strings.ReplaceAll(result, v, "***")
	}
	return result
}

// WrapWriter wraps an io.Writer to mask sensitive values.
func (f *MaskingFilter) WrapWriter(w io.Writer) io.Writer {
	return &maskedWriter{
		underlying: w,
		filter:     f,
	}
}

// maskedWriter is an io.Writer that masks sensitive values before writing.
type maskedWriter struct {
	underlying io.Writer
	filter     *MaskingFilter
}

func (mw *maskedWriter) Write(p []byte) (int, error) {
	masked := mw.filter.Mask(string(p))
	_, err := mw.underlying.Write([]byte(masked))
	if err != nil {
		return 0, err
	}
	// Return the original length so callers don't see a short write
	return len(p), nil
}
