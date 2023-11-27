package graphology

import (
	"encoding/json"
	"os"
	"strconv"
	"sync"

	"github.com/psidex/nomad/internal/graphs"
	. "github.com/psidex/nomad/internal/lib"
)

// Graphology defines a CliGraphProvider that renders Graphology data to a JSON file. See
// ../graphologyws/graphologyws.go for an explanation of the code.
type Graphology struct {
	mu              *sync.Mutex
	hasher          *StrHasher
	graphologyGraph *SerializedGraph
	nodes           map[string]*Node
	seenNodes       Set
	seenEdges       Set
	edgeCount       int
}

var _ graphs.CliGraphProvider = (*Graphology)(nil)

func NewGraphology() *Graphology {
	return &Graphology{
		mu:              &sync.Mutex{},
		hasher:          NewStrHasher(),
		graphologyGraph: &SerializedGraph{},
		nodes:           make(map[string]*Node),
		seenNodes:       NewSet(),
		seenEdges:       NewSet(),
		edgeCount:       0,
	}
}

func (g *Graphology) AddHostnameConnection(fromHost, toHost string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	fromHostId := strconv.Itoa(g.hasher.Hash(fromHost))
	if !g.seenNodes.Contains(fromHostId) {
		g.seenNodes.Add(fromHostId)
		fromHostNode := Node{Key: fromHostId}
		fromHostNode.Attributes = NodeAttributes{
			X: 0, Y: 0, Size: 2,
			Label: fromHost, Color: "blue",
		}
		g.nodes[fromHostId] = &fromHostNode
	} else {
		if g.nodes[fromHostId].Attributes.Size < 10 {
			g.nodes[fromHostId].Attributes.Size += 0.2
		}
	}

	toHostId := strconv.Itoa(g.hasher.Hash(toHost))
	if !g.seenNodes.Contains(toHostId) {
		g.seenNodes.Add(toHostId)
		toHostNode := Node{Key: toHostId}
		toHostNode.Attributes = NodeAttributes{
			X: 0, Y: 0, Size: 2,
			Label: toHost, Color: "blue",
		}
		g.nodes[toHostId] = &toHostNode
	}

	edgeStr := fromHostId + "\t" + toHostId
	inverseEdgeStr := toHostId + "\t" + fromHostId

	if !g.seenEdges.Contains(edgeStr) && !g.seenEdges.Contains(inverseEdgeStr) {
		g.seenEdges.Add(edgeStr)
		g.edgeCount++
		edge := Edge{
			Key:    strconv.Itoa(g.edgeCount),
			Source: fromHostId,
			Target: toHostId,
			Attributes: EdgeAttributes{
				Size: 2,
			},
		}
		g.graphologyGraph.Edges = append(g.graphologyGraph.Edges, edge)
	}
}

func (g Graphology) enrichGraph() {
	// Add in all our nodes
	for _, node := range g.nodes {
		g.graphologyGraph.Nodes = append(g.graphologyGraph.Nodes, *node)
	}
}

func (g Graphology) RenderToFile(filename string) error {
	filename = filename + ".json"

	g.mu.Lock()
	defer g.mu.Unlock()

	g.enrichGraph()

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	marshalled, err := json.Marshal(g.graphologyGraph)
	if err != nil {
		return err
	}

	_, err = file.Write(marshalled)
	if err != nil {
		return err
	}

	return nil
}
