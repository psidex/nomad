package vis

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/psidex/nomad/internal/graphs"
	. "github.com/psidex/nomad/internal/lib"
)

// Vis defines a CliGraphProvider that renders to a HTML file which "replays" the crawl
// using vis.js.
type Vis struct {
	mu        *sync.Mutex
	hasher    *StrHasher
	seenNodes Set
	seenEdges Set
	output    string
}

var _ graphs.CliGraphProvider = (*Vis)(nil)

func NewVis() *Vis {
	return &Vis{
		mu:        &sync.Mutex{},
		hasher:    NewStrHasher(),
		seenNodes: NewSet(),
		seenEdges: NewSet(),
		output:    "",
	}
}

// FIXME: This algorithm is quite inefficient:
//   - Renders JSON before checking if it needs to
//   - Stores rendered JSON in the node & edge sets
//   - Doesn't check inverse edges before adding to output

func (v *Vis) AddHostnameConnection(fromHost, toHost string) {
	v.mu.Lock()
	defer v.mu.Unlock()

	// Get the unique ID for this host.
	fromHostId := v.hasher.Hash(fromHost)

	// Create the data structure to be marshalled to JSON.
	fromHostNode := newNode()
	fromHostNode.Data = nodeData{ID: fromHostId, Label: fromHost}

	// Marshall to JSON and get that as a string.
	fromHostNodeJson, err := json.Marshal(fromHostNode)
	if err != nil {
		// For now, if we encounter an error, just abandon this process.
		return
	}
	fromHostNodeJsonStr := string(fromHostNodeJson)

	// If we haven't already seen this JSON.
	if !v.seenNodes.Contains(fromHostNodeJsonStr) {
		// Remember that we've seen it, and add it to our output.
		v.seenNodes.Add(fromHostNodeJsonStr)
		v.output += fmt.Sprintf("\n%s,", fromHostNodeJsonStr)
	}

	toHostId := v.hasher.Hash(toHost)
	toHostNode := newNode()
	toHostNode.Data = nodeData{ID: toHostId, Label: toHost}
	toHostNodeJson, err := json.Marshal(toHostNode)
	if err != nil {
		return
	}
	toHostNodeJsonStr := string(toHostNodeJson)
	if !v.seenNodes.Contains(toHostNodeJsonStr) {
		v.seenNodes.Add(toHostNodeJsonStr)
		v.output += fmt.Sprintf("\n%s,", toHostNodeJsonStr)
	}

	edge := newEdge()
	edge.Data = edgeData{From: fromHostId, To: toHostId}
	edgeJson, err := json.Marshal(edge)
	if err != nil {
		return
	}
	edgeJsonStr := string(edgeJson)
	if !v.seenEdges.Contains(edgeJsonStr) {
		v.seenEdges.Add(edgeJsonStr)
		v.output += fmt.Sprintf("\n%s,", edgeJsonStr)
	}
}

func (v Vis) RenderToFile(filename string) error {
	filename = filename + ".html"

	v.mu.Lock()
	defer v.mu.Unlock()

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(fmt.Sprintf(html, v.output))
	if err != nil {
		return err
	}

	return nil
}
