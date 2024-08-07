package graphology

import (
	"fmt"
)

type nodeAttributes struct {
	// The frontend will determine the commented attributes for us.
	// X     float64 `json:"x"`
	// Y     float64 `json:"y"`
	// Size  float64 `json:"size"`
	Label string `json:"label"`
	// Color string  `json:"color"`
}

type node struct {
	Key        string         `json:"key"`
	Attributes nodeAttributes `json:"attributes"`
}

func (n node) toNodeJson() []byte {
	// n.Attributes should have a Label set.
	return []byte(fmt.Sprintf(
		`{"type": "node", "data": {"key": "%s", "attributes": {"label": "%s"}}}`,
		n.Key, n.Attributes.Label,
	))
}

func (n node) toNodeUpdateJson() []byte {
	// Only n.Key needs to be set.
	return []byte(fmt.Sprintf(
		`{"type": "nodeupdate", "data": {"key": "%s"}}`, n.Key,
	))
}

type edge struct {
	Key    string `json:"key"`
	Source string `json:"source"`
	Target string `json:"target"`
}

func (e edge) toEdgeJson() []byte {
	return []byte(fmt.Sprintf(
		`{"type": "edge", "data": {"from": "%s", "to": "%s"}}`, e.Source, e.Target,
	))
}
