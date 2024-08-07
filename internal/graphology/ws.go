package graphology

import (
	"log"
	"strconv"
	"sync"

	"github.com/gorilla/websocket"

	"github.com/psidex/nomad/internal/lib"
	. "github.com/psidex/nomad/internal/lib"
)

// GraphologyWs defines a WebsocketGraphProvider that renders Graphology data for
// nomad-frontend as JSON and streams it over a websocket.
type GraphologyWs struct {
	mu     *sync.Mutex
	hasher *StrHasher
	ws     lib.ThreadSafeWebSocket
	// Keep track of nodes and edges so we know what's been seen.
	seenNodes Set
	seenEdges Set
	edgeCount int
}

// All of the websocket messages sent by GraphologyWs will be text.
var t = websocket.TextMessage

func NewGraphologyWs(ws lib.ThreadSafeWebSocket) *GraphologyWs {
	return &GraphologyWs{
		mu:        &sync.Mutex{},
		hasher:    NewStrHasher(),
		ws:        ws,
		seenNodes: NewSet(),
		seenEdges: NewSet(),
		edgeCount: 0,
	}
}

func (g *GraphologyWs) AddHostnameConnection(fromHost, toHost string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Get the unique ID for this host.
	fromHostId := strconv.Itoa(g.hasher.Hash(fromHost))
	// Create the data structure.
	fromHostNode := node{Key: fromHostId}

	if !g.seenNodes.Contains(fromHostId) {
		// This is a new node.
		g.seenNodes.Add(fromHostId)

		// Add a label to the data structure.
		fromHostNode.Attributes = nodeAttributes{Label: fromHost}

		// Inform the frontend of the new node.
		if err := g.ws.WriteMessage(t, fromHostNode.toNodeJson()); err != nil {
			log.Print("ws.WriteMessage err:", err)
		}
	} else {
		// Inform the frontend that this has been seen again.
		if err := g.ws.WriteMessage(t, fromHostNode.toNodeUpdateJson()); err != nil {
			log.Print("ws.WriteMessage err:", err)
		}
	}

	toHostId := strconv.Itoa(g.hasher.Hash(toHost))
	if !g.seenNodes.Contains(toHostId) {
		g.seenNodes.Add(toHostId)
		toHostNode := node{Key: toHostId}
		toHostNode.Attributes = nodeAttributes{toHost}
		if err := g.ws.WriteMessage(t, toHostNode.toNodeJson()); err != nil {
			log.Print("ws.WriteMessage err:", err)
		}
	}

	// Check if we've seen this edge before in either direction - use tab as a separator
	// as it's impossible to appear in the IDs.
	edgeStr := fromHostId + "\t" + toHostId
	inverseEdgeStr := toHostId + "\t" + fromHostId

	if !g.seenEdges.Contains(edgeStr) && !g.seenEdges.Contains(inverseEdgeStr) {
		// Only need to add the first because we check for both each time
		g.seenEdges.Add(edgeStr)
		g.edgeCount++

		edge := edge{strconv.Itoa(g.edgeCount), fromHostId, toHostId}
		if err := g.ws.WriteMessage(t, edge.toEdgeJson()); err != nil {
			log.Print("ws.WriteMessage err:", err)
		}
	}
}

func (g GraphologyWs) NotifyStartCrawl(workerId uint, hostname string) {
	hostnameId := g.hasher.Hash(hostname)
	err := g.ws.WriteMessage(t, startCrawlNotification(workerId, hostnameId))
	if err != nil {
		log.Print("NotifyStartCrawl ws.WriteMessage err:", err)
	}
}

func (g GraphologyWs) NotifyEndCrawl(workerId uint, hostname string, deadEnd bool) {
	hostnameId := g.hasher.Hash(hostname)
	err := g.ws.WriteMessage(t, endCrawlNotification(workerId, hostnameId, deadEnd))
	if err != nil {
		log.Print("NotifyEndCrawl ws.WriteMessage err:", err)
	}
}
