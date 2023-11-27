package graphology

type NodeAttributes struct {
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Size  float64 `json:"size"`
	Label string  `json:"label"`
	Color string  `json:"color"`
}

type Node struct {
	Key        string         `json:"key"`
	Attributes NodeAttributes `json:"attributes"`
}

type EdgeAttributes struct {
	Size int `json:"size"`
}

type Edge struct {
	Key        string         `json:"key"`
	Source     string         `json:"source"`
	Target     string         `json:"target"`
	Attributes EdgeAttributes `json:"attributes"`
}

type SerializedGraph struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}
