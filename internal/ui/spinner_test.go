package ui

import (
	"bytes"
	"testing"
)

func TestSpinnerFrames(t *testing.T) {
	s := NewSpinner(&bytes.Buffer{})
	frames := s.Frames()

	expectedFrames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	if len(frames) != len(expectedFrames) {
		t.Fatalf("expected %d frames, got %d", len(expectedFrames), len(frames))
	}
	for i, f := range frames {
		if f != expectedFrames[i] {
			t.Errorf("frame[%d] = %q, want %q", i, f, expectedFrames[i])
		}
	}
}

func TestSpinnerFrame(t *testing.T) {
	s := NewSpinner(&bytes.Buffer{})
	// Initial frame should be the first braille character.
	frame := s.Frame()
	if frame != "⠋" {
		t.Errorf("initial frame = %q, want %q", frame, "⠋")
	}
}

func TestSpinnerStartStop(t *testing.T) {
	var buf bytes.Buffer
	s := NewSpinner(&buf)

	s.Start("test")
	// Give the spinner a brief moment to render.
	// We just verify Start/Stop don't panic or deadlock.
	s.Stop()

	// After stop, the buffer should contain the clear sequence.
	output := buf.String()
	if len(output) == 0 {
		t.Error("spinner should have written output")
	}
}

func TestSpinnerDoubleStart(t *testing.T) {
	var buf bytes.Buffer
	s := NewSpinner(&buf)

	s.Start("test1")
	s.Start("test2") // Should be a no-op since already active.
	s.Stop()
}

func TestSpinnerDoubleStop(t *testing.T) {
	var buf bytes.Buffer
	s := NewSpinner(&buf)

	s.Start("test")
	s.Stop()
	s.Stop() // Should be a no-op since already stopped.
}

func TestSpinnerStopWithoutStart(t *testing.T) {
	var buf bytes.Buffer
	s := NewSpinner(&buf)
	s.Stop() // Should be a no-op.
}
