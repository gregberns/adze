package secrets

import (
	"bytes"
	"testing"
)

func TestMaskingFilter_RegisterAndMask(t *testing.T) {
	f := NewMaskingFilter()
	f.Register("mysecret123")

	got := f.Mask("The value is mysecret123 here")
	expected := "The value is *** here"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestMaskingFilter_MultipleValues(t *testing.T) {
	f := NewMaskingFilter()
	f.Register("secret_one")
	f.Register("secret_two")

	got := f.Mask("Got secret_one and secret_two in output")
	expected := "Got *** and *** in output"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestMaskingFilter_EmptyStringNotRegistered(t *testing.T) {
	f := NewMaskingFilter()
	f.Register("")

	// Should not panic or replace anything
	got := f.Mask("some output")
	if got != "some output" {
		t.Errorf("expected 'some output', got %q", got)
	}
}

func TestMaskingFilter_NoRegisteredValues(t *testing.T) {
	f := NewMaskingFilter()

	got := f.Mask("nothing to mask here")
	if got != "nothing to mask here" {
		t.Errorf("expected unchanged string, got %q", got)
	}
}

func TestMaskingFilter_MultipleOccurrences(t *testing.T) {
	f := NewMaskingFilter()
	f.Register("tok")

	got := f.Mask("tok is repeated: tok and tok")
	expected := "*** is repeated: *** and ***"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestMaskingFilter_WrapWriter(t *testing.T) {
	f := NewMaskingFilter()
	f.Register("hideme")

	var buf bytes.Buffer
	w := f.WrapWriter(&buf)

	msg := "The token hideme is secret"
	n, err := w.Write([]byte(msg))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != len(msg) {
		t.Errorf("expected n=%d, got n=%d", len(msg), n)
	}

	got := buf.String()
	expected := "The token *** is secret"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestMaskingFilter_WrapWriter_MultipleWrites(t *testing.T) {
	f := NewMaskingFilter()
	f.Register("abc")

	var buf bytes.Buffer
	w := f.WrapWriter(&buf)

	w.Write([]byte("first abc "))
	w.Write([]byte("second abc"))

	got := buf.String()
	expected := "first *** second ***"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}
