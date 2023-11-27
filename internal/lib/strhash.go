package lib

import "sync"

// StrHasher gives a unique int to each unique string, it doesn't actually hash the
// strings, it stores a map of [string]int - a similar effect to hashing but uses more
// memory & less cpu.
// TODO: Maybe actually use a basic hashing algo, maybe cespare/xxhash?
type StrHasher struct {
	mu      *sync.Mutex
	ids     map[string]int
	counter int
}

func NewStrHasher() *StrHasher {
	return &StrHasher{
		mu:      &sync.Mutex{},
		ids:     make(map[string]int),
		counter: 0,
	}
}

// Hash returns a unique int for each unique string.
func (s *StrHasher) Hash(str string) (id int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var ok bool
	if id, ok = s.ids[str]; !ok {
		id = s.counter + 1
		s.counter = id
		s.ids[str] = id
	}
	return id
}
