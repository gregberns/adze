package ui

import (
	"fmt"
	"io"
	"sync"
	"time"
)

// spinnerFrames are the braille pattern characters used for the spinner animation.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Spinner handles TTY spinner animation.
type Spinner struct {
	frames  []string
	current int
	active  bool
	done    chan struct{}
	writer  io.Writer
	mu      sync.Mutex
	label   string
}

// NewSpinner creates a new Spinner that writes to w.
func NewSpinner(w io.Writer) *Spinner {
	return &Spinner{
		frames: spinnerFrames,
		writer: w,
	}
}

// Start begins the spinner animation with the given label. The spinner
// renders at 100ms intervals until Stop is called.
func (s *Spinner) Start(label string) {
	s.mu.Lock()
	if s.active {
		s.mu.Unlock()
		return
	}
	s.active = true
	s.current = 0
	s.label = label
	s.done = make(chan struct{})
	s.mu.Unlock()

	go s.run()
}

// Stop halts the spinner animation and clears the spinner line.
func (s *Spinner) Stop() {
	s.mu.Lock()
	if !s.active {
		s.mu.Unlock()
		return
	}
	s.active = false
	s.mu.Unlock()

	<-s.done

	// Clear the spinner line: move to start of line, clear to end.
	fmt.Fprintf(s.writer, "\r\033[K")
}

// Frame returns the current spinner frame character. This is useful for testing.
func (s *Spinner) Frame() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.frames[s.current%len(s.frames)]
}

// Frames returns the list of spinner frame characters.
func (s *Spinner) Frames() []string {
	return s.frames
}

func (s *Spinner) run() {
	defer close(s.done)

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		s.mu.Lock()
		if !s.active {
			s.mu.Unlock()
			return
		}
		frame := s.frames[s.current%len(s.frames)]
		label := s.label
		s.current++
		s.mu.Unlock()

		fmt.Fprintf(s.writer, "\r\033[K%s %s...", frame, label)

		<-ticker.C
	}
}
