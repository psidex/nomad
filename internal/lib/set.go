package lib

import (
	"sync"
)

// Set is thread-safe and can be passed by value.
type Set struct {
	data map[string]struct{}
	mu   *sync.RWMutex
}

func NewSet() Set {
	return Set{
		data: make(map[string]struct{}),
		mu:   &sync.RWMutex{},
	}
}

func (s Set) Add(elem string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[elem] = struct{}{}
}

func (s Set) Remove(elem string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, elem)
}

func (s Set) Contains(elem string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.data[elem]
	return exists
}

func (s Set) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data)
}

func (s Set) AsSlice() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	elements := make([]string, 0, len(s.data))
	for elem := range s.data {
		elements = append(elements, elem)
	}

	return elements
}
