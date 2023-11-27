package graphs

import (
	"io"
	"os"
	"sync"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/components"
	"github.com/go-echarts/go-echarts/v2/opts"

	. "github.com/psidex/nomad/internal/lib"
)

// ECharts defines a CliGraphProvider that renders a go-echarts HTML file.
type ECharts struct {
	mu        *sync.Mutex
	hostnames Set
	edges     Set
	nodes     []opts.GraphNode
	links     []opts.GraphLink
}

var _ CliGraphProvider = (*ECharts)(nil)

func NewECharts() *ECharts {
	return &ECharts{
		mu:        &sync.Mutex{},
		hostnames: NewSet(),
		edges:     NewSet(),
		nodes:     []opts.GraphNode{},
		links:     []opts.GraphLink{},
	}
}

func (e ECharts) getNewNodesAndEdges(fromHost, toHost string) ([]opts.GraphNode, []opts.GraphLink) {
	newNodes := []opts.GraphNode{}
	newLinks := []opts.GraphLink{}

	if !e.hostnames.Contains(fromHost) {
		e.hostnames.Add(fromHost)
		newNodes = append(newNodes, opts.GraphNode{
			Name: fromHost,
		})
	}

	if !e.hostnames.Contains(toHost) {
		e.hostnames.Add(toHost)
		newNodes = append(newNodes, opts.GraphNode{
			Name: toHost,
		})
	}

	edge := fromHost + "\t" + toHost
	inverseEdge := toHost + "\t" + fromHost

	if fromHost != toHost && !e.edges.Contains(edge) && !e.edges.Contains(inverseEdge) {
		e.edges.Add(edge)
		newLinks = append(newLinks, opts.GraphLink{
			Source: fromHost,
			Target: toHost,
		})
	}

	return newNodes, newLinks
}

func (e *ECharts) AddHostnameConnection(fromHost, toHost string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	newNodes, newLinks := e.getNewNodesAndEdges(fromHost, toHost)

	e.nodes = append(e.nodes, newNodes...)
	e.links = append(e.links, newLinks...)
}

func (e ECharts) RenderToFile(filename string) error {
	filename = filename + ".html"

	e.mu.Lock()
	defer e.mu.Unlock()

	page := components.NewPage()
	page.AddCharts(graphBase(e.nodes, e.links))

	f, err := os.Create(filename)
	if err != nil {
		return err
	}

	return page.Render(io.MultiWriter(f))
}

func graphBase(nodes []opts.GraphNode, links []opts.GraphLink) *charts.Graph {
	graph := charts.NewGraph()
	graph.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			PageTitle: "nomad results",
			Height:    "100vh",
			Width:     "100vw",
		}),
		charts.WithLegendOpts(opts.Legend{
			Show: opts.Bool(false),
		}),
		charts.WithTooltipOpts(opts.Tooltip{
			Show: opts.Bool(true),
		}),
	)
	graph.AddSeries(
		"graph",
		nodes,
		links,
		charts.WithGraphChartOpts(
			opts.GraphChart{
				Draggable: opts.Bool(true),
				Roam:      opts.Bool(true),
				Force:     &opts.GraphForce{Repulsion: 400},
			},
		),
		charts.WithLabelOpts(opts.Label{
			Show:     opts.Bool(true),
			Color:    "black",
			Position: "top",
		}),
	)
	return graph
}
