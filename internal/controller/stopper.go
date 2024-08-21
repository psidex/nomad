package controller

import (
	"sync"
)

// TODO: Move to lib?

// Stopper helps with stopping the execution of things. Don't make copies! WARNING: If
// deferring Stop(), wrap it in a closure so that your Stopper pointer isn't
// dereferenced, see https://victoriametrics.com/blog/defer-in-go/.
// TODO: Confirm this warning is correct, reproduce on go playground?
type Stopper struct {
	tidy func()
	// Select on this to know when stopped, or call IsStopped to check.
	// Don't do anything else to this other than selecting!
	Ch chan struct{}
	// Call this as many times as you want to stop
	Stop func()
}

// NewStopper creates a new stopper. Pass a tidy func to run on stopping.
func NewStopper(tidy func()) *Stopper {
	s := &Stopper{tidy: tidy}
	s.Reset()
	return s
}

// Reset will un-stop the stopper if needed
func (s *Stopper) Reset() {
	s.Ch = make(chan struct{})
	s.Stop = sync.OnceFunc(func() {
		close(s.Ch)
		s.tidy()
	})
}

func (s *Stopper) IsStopped() bool {
	select {
	case <-s.Ch:
		return true
	default:
	}
	return false
}
