package graphs

import (
	"encoding/json"
	"os"
	"sync"

	. "github.com/psidex/nomad/internal/lib"
)

// HostnameGraph defines a CliGraphProvider that keeps track of hostname connections using
// a map[string]Set and renders this to a JSON file. It does not de-duplicate edges.
type HostnameGraph struct {
	mu                *sync.RWMutex
	hostname2hostname map[string]Set
}

var _ CliGraphProvider = (*HostnameGraph)(nil)

func NewHostnameGraph() HostnameGraph {
	return HostnameGraph{
		mu:                &sync.RWMutex{},
		hostname2hostname: make(map[string]Set),
	}
}

func (h HostnameGraph) AddHostnameConnection(fromHost, toHost string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.hostname2hostname[fromHost]; !ok {
		h.hostname2hostname[fromHost] = NewSet()
	}
	h.hostname2hostname[fromHost].Add(toHost)
}

func (h HostnameGraph) toJson() ([]byte, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	slicedSets := make(map[string][]string)
	for key, value := range h.hostname2hostname {
		slicedSets[key] = value.AsSlice()
	}

	return json.MarshalIndent(slicedSets, "", "  ")
}

func (h HostnameGraph) RenderToFile(filename string) error {
	filename = filename + ".json"

	jsonData, err := h.toJson()
	if err != nil {
		return err
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(jsonData)
	if err != nil {
		return err
	}

	return nil
}
