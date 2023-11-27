package vis

type nodeData struct {
	ID    int    `json:"id"`
	Label string `json:"label"`
}

type node struct {
	Type string   `json:"type"` // always "node"
	Data nodeData `json:"data"`
}

func newNode() node {
	return node{Type: "node"}
}

type edgeData struct {
	From int `json:"from"`
	To   int `json:"to"`
}

type edge struct {
	Type string   `json:"type"` // always "edge"
	Data edgeData `json:"data"`
}

func newEdge() edge {
	return edge{Type: "edge"}
}
