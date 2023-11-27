package lib

import (
	"math/rand"
	"sync"
)

// Queue is thread-safe and should be held as a pointer.
type Queue struct {
	items []string
	mu    *sync.RWMutex
}

func NewQueue() *Queue {
	return &Queue{
		items: []string{},
		mu:    &sync.RWMutex{},
	}
}

func (q *Queue) Enqueue(item string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = append(q.items, item)
}

// Dequeue pops from the front of the queue (FIFO).
func (q *Queue) Dequeue() string {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.items) == 0 {
		return ""
	}

	item := q.items[0]
	q.items = q.items[1:]
	return item
}

// RandomDequeue pops a random item from the queue.
func (q *Queue) RandomDequeue() string {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.items) == 0 {
		return ""
	}

	i := rand.Intn(len(q.items))
	item := q.items[i]

	q.items = append(q.items[:i], q.items[i+1:]...)

	return item

}

func (q Queue) Size() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.items)
}
