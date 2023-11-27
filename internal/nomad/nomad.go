package nomad

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/corpix/uarand"
	"golang.org/x/net/html"

	"github.com/psidex/nomad/internal/frontier"
	"github.com/psidex/nomad/internal/graphs"
	"github.com/psidex/nomad/internal/lib"
)

// TODO: Configurable "stealth mode" - if not in stealth mode, check robots.txt, etc.

type Config struct {
	WorkerCooldown lib.Duration `json:"workerCooldown"`
	WorkerCount    uint         `json:"workerCount"`
	InitialUrls    []string     `json:"initialUrls"`
	RandomCrawl    bool         `json:"randomCrawl"`
}

type Nomad struct {
	// Set in NewNomad(...).
	cfg    Config
	client *http.Client
	graph  graphs.GraphProvider
	// Set / reset at the start of Run().
	frontier frontier.Frontier
	cancel   chan struct{}
	wg       *sync.WaitGroup
}

func NewNomad(cfg Config, hc *http.Client, gp graphs.GraphProvider) *Nomad {
	return &Nomad{
		cfg:    cfg,
		client: hc,
		graph:  gp,
	}
}

// Cancel gracefully stops all the workers once they're finished processing their
// current URL, and blocks until all the goroutines are exited.
func (n *Nomad) Cancel() {
	close(n.cancel)
	n.wg.Wait()
}

func (n *Nomad) Run() error {
	n.frontier = frontier.NewFrontier(n.cfg.RandomCrawl)
	n.cancel = make(chan struct{})
	n.wg = &sync.WaitGroup{}

	for _, initialUrl := range n.cfg.InitialUrls {
		toAdd, err := getHostnameAsUrl(initialUrl)
		if err != nil {
			return err
		}
		n.frontier.AddUrl(toAdd)
	}

	for i := uint(1); i <= n.cfg.WorkerCount; i++ {
		n.wg.Add(1)
		go n.worker(i)
	}

	return nil
}

func (n Nomad) worker(id uint) {
	defer n.wg.Done()

	for {
		select {
		case <-n.cancel:
			log.Printf("{%d} Canceled\n", id)
			return
		default:
		}

		log.Printf("{%d} Loop, frontier size: %d\n", id, n.frontier.Size())

		if currentlUrl := n.frontier.PopUrl(); currentlUrl != "" {
			n.workOnUrl(id, currentlUrl)
		} else {
			log.Printf("{%d} Sleeping on empty frontier\n", id)
		}

		time.Sleep(n.cfg.WorkerCooldown.Duration)
	}
}

func (n Nomad) workOnUrl(id uint, currentlUrl string) {
	log.Printf("{%d} Processing URL %s\n", id, currentlUrl)

	currentHostname, err := getHostname(currentlUrl)
	if err != nil {
		log.Printf("{%d} Could not get current URL hostname, err: %v\n", id, err)
		return
	}

	var urls []string

	if g, ok := n.graph.(graphs.WebsocketGraphProvider); ok {
		g.NotifyStartCrawl(id, currentHostname)
		defer func() {
			g.NotifyEndCrawl(id, currentHostname, len(urls) == 0)
		}()
	}

	urls, err = n.getUrls(currentlUrl)
	if err != nil {
		log.Printf("{%d} Could not get URLs, err: %v\n", id, err)
		return
	}

	log.Printf("{%d} Found %d URLs\n", id, len(urls))

	for _, foundUrl := range urls {
		foundHostname, err := getHostname(foundUrl)
		if err != nil {
			log.Printf("{%d} Could not get found URLs hostname, err: %v\n", id, err)
			continue
		}
		if foundHostname == currentHostname {
			// We don't care about self referential links.
			continue
		}

		// Get the hostname as the URL - we don't want to follow specific URLs,
		// just scrape as many hostnames as possible.
		foundHostnameAsUrl, err := getHostnameAsUrl(foundUrl)
		if err != nil {
			log.Printf("{%d} Could not get found URLs hostname as a URL, foundUrl: %s, err: %v\n", id, foundUrl, err)
			continue
		}

		// URL will be ignored by AddUrl if we've already seen it.
		if added := n.frontier.AddUrl(foundHostnameAsUrl); added {
			n.graph.AddHostnameConnection(currentHostname, foundHostname)
		}
	}
}

func (n Nomad) getUrls(urlStr string) ([]string, error) {
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", uarand.GetRandom())

	resp, err := n.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("got non-OK status code: %v", resp.StatusCode)
	}

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, err
	}

	baseURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}

	return extractURLs(doc, baseURL), nil
}
