package main

import (
	"log"
	"net/http"
	"time"

	"github.com/psidex/nomad/internal/graphs"
	"github.com/psidex/nomad/internal/graphs/graphology"
	"github.com/psidex/nomad/internal/graphs/vis"
	"github.com/psidex/nomad/internal/lib"
	"github.com/psidex/nomad/internal/nomad"
)

// TODO: These should be configurable
var (
	initialUrls            = []string{"https://www.france.fr/"}
	runtime                = time.Second * 15
	workerCooldown         = lib.DurationFrom(time.Millisecond * 5000)
	workerCount       uint = 3
	graphProvider          = "vis"
	httpClientTimeout      = time.Second * 10
	randomCrawl            = false
	filename               = "nomaddata"
)

func main() {
	var chosenGraph graphs.CliGraphProvider
	switch graphProvider {
	case "echarts":
		chosenGraph = graphs.NewECharts()
	case "vis":
		chosenGraph = vis.NewVis()
	case "json":
		chosenGraph = graphs.NewHostnameGraph()
	case "graphology":
		chosenGraph = graphology.NewGraphology()
	default:
		log.Fatalf("unknown graph provider: %s", graphProvider)
	}

	n := nomad.NewNomad(
		nomad.Config{
			WorkerCooldown: workerCooldown,
			WorkerCount:    workerCount,
			InitialUrls:    initialUrls,
			RandomCrawl:    randomCrawl,
		},
		&http.Client{
			Timeout: httpClientTimeout,
		},
		chosenGraph,
	)

	if err := n.Run(); err != nil {
		panic(err)
	}

	time.Sleep(runtime)
	n.Cancel()

	if err := chosenGraph.RenderToFile(filename); err != nil {
		panic(err)
	}
}
