package frontier

import (
	"sync"

	. "github.com/psidex/nomad/internal/lib"
)

type Frontier struct {
	random bool
	queue  *Queue
	// visitedMu is for synchronicity rather than thread safety, it's possible for 2 of
	// the same URL to exist the queue next to eachother, and for 2 PopUrl calls to
	// happen at exactly the same time; without the mutex and with some bad luck, a
	// duplicated URL could sneak past between checking if it's visited and adding to
	// the visited set.
	visitedMu *sync.Mutex
	visited   Set
}

// NewFrontier creates a new Frontier. The random parameter determines if the popped
// URLs are random or FIFO.
func NewFrontier(random bool) Frontier {
	return Frontier{
		random:    random,
		queue:     NewQueue(),
		visitedMu: &sync.Mutex{},
		visited:   NewSet(),
	}
}

// AddUrl adds a URL to the frontier, returns true if added, false if it's already been
// visited.
func (f Frontier) AddUrl(url string) bool {
	// visitedMu is probably less important here, but might as well use it.
	f.visitedMu.Lock()
	defer f.visitedMu.Unlock()
	if f.visited.Contains(url) {
		return false
	}
	f.queue.Enqueue(url)
	return true
}

// PopUrl gets an unvisited URL from the frontier, can return an empty string if there's
// nothing to pop.
func (f Frontier) PopUrl() (url string) {
	f.visitedMu.Lock()
	defer f.visitedMu.Unlock()
	for {
		if f.random {
			url = f.queue.RandomDequeue()
		} else {
			url = f.queue.Dequeue()
		}
		if url != "" && f.visited.Contains(url) {
			// 2 of the same URL can appear in the queue if, for example, 2 of the same
			// URL are found on the same page. We could prevent this by checking the
			// contents of f.queue in AddUrl but checking a slice for a value is O(n)...
			continue
		}
		f.visited.Add(url)
		break
	}
	return url
}

// Size returns the size of the frontier, not accounting for entries that may have
// already been visited.
func (f Frontier) Size() int {
	return f.queue.Size()
}
