package controller

import (
	"sync"
)

// TODO: Move to lib

// Stopper helps with sychronising a stop, thread safe via a mutex. Don't make copies!
type Stopper struct {
	mu      sync.Mutex
	stopped func()
	// Select on this to know when stopped, or call Stopper.IsStopped to check.
	// Don't do anything else to this other than selecting!
	Ch chan struct{}
}

// NewStopper creates a new stopper. Pass a function to run on stopping.
func NewStopper(stopped func()) *Stopper {
	return &Stopper{sync.Mutex{}, stopped, make(chan struct{})}
}

// UnStop will un-stop the stopper, if needed. Blocks any other UnStop() or Stop()
// calls.
func (s *Stopper) UnStop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.IsStopped() {
		s.Ch = make(chan struct{})
	}
}

// Stop the stopper. Blocks any other UnStop() or Stop() calls. If this stops, the saved
// function will be called.
func (s *Stopper) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.IsStopped() {
		close(s.Ch)
		s.stopped()
	}
}

// IsStopped checks if the stopper is stopped. Directly checks the goroutine,
func (s *Stopper) IsStopped() bool {
	select {
	case <-s.Ch:
		return true
	default:
	}
	return false
}
