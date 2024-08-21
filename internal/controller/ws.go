package controller

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/psidex/nomad/internal/frontier"
	"github.com/psidex/nomad/internal/graphology"
	"github.com/psidex/nomad/internal/lib"

	"github.com/psidex/nomad/internal/controller/pb"
)

var (
	upgrader = websocket.Upgrader{}
)

type SessionConfig struct {
	Runtime           lib.Duration `json:"runtime"`
	HttpClientTimeout lib.Duration `json:"httpClientTimeout"` // unused
	WorkerCooldown    lib.Duration `json:"workerCooldown"`    // unused
	WorkerCount       uint         `json:"workerCount"`       // unused
	RandomCrawl       bool         `json:"randomCrawl"`       // unused
	InitialUrls       []string     `json:"initialUrls"`
}

type WebServer struct {
	logger          *slog.Logger
	frontier        *frontier.Frontier
	stopper         *Stopper
	scrapedDataChan chan *pb.ScrapedData
	//
	frontend     *graphology.GraphologyWs
	clientActive atomic.Bool
}

func init() {
	upgrader.CheckOrigin = func(r *http.Request) bool { return true }
}

// TODO: Tidy signature
func NewWebServer(
	logger *slog.Logger,
	frontier *frontier.Frontier,
	stopper *Stopper,
	scrapedDataChan chan *pb.ScrapedData,
) *WebServer {
	return &WebServer{
		logger:          logger,
		frontier:        frontier,
		stopper:         stopper,
		scrapedDataChan: scrapedDataChan,
		clientActive:    atomic.Bool{},
	}
}

func (w *WebServer) NotifyStartCrawl(workerId int32, hostname string) {
	if w.frontend != nil {
		w.frontend.NotifyStartCrawl(uint(workerId), hostname)
	}
}

func (w *WebServer) Session(respWriter http.ResponseWriter, req *http.Request) {
	if w.clientActive.Load() {
		w.logger.Warn("Rejecting client as we already have one")
		_, _ = respWriter.Write([]byte("go away"))
		return
	}

	defer w.clientActive.Store(false)
	w.clientActive.Store(true)

	if !w.stopper.IsStopped() {
		w.logger.Error("Rejecting client as stopper is not currently stopped")
		_, _ = respWriter.Write([]byte("something broke"))
		return
	}

	w.stopper.Reset()
	// Must defer after Reset() so the evaluated receiver contains the correct values
	defer w.stopper.Stop()

	c, err := upgrader.Upgrade(respWriter, req, nil)
	if err != nil {
		w.logger.Error("ws upgrade err:", "error", err)
		return
	}
	defer c.Close()

	ws := lib.NewThreadSafeWebSocket(c)

	_, msg, err := ws.ReadMessage()
	if err != nil {
		w.logger.Error("ws cfg read err:", "error", err)
		return
	}

	cfg := &SessionConfig{}
	if err = json.Unmarshal(msg, cfg); err != nil {
		w.logger.Error("ws cfg unmarshal err:", "error", err)
		return
	}

	w.frontend = graphology.NewGraphologyWs(ws)

	for _, initialUrl := range cfg.InitialUrls {
		toAdd, err := getHostnameAsUrl(initialUrl)
		if err != nil {
			w.logger.Error("getHostnameAsUrl err:", "error", err)
			return
		}
		w.frontier.AddUrl(toAdd)
	}

	go func() {
		// Take scraped data, process it, send info to front end if required
		for {
			select {
			case <-w.stopper.Ch:
				return
			case work := <-w.scrapedDataChan:
				w.logger.Debug("handle work loop got work", "work", work)
				if w.frontend != nil {
					scrapedHostname, _ := getHostname(work.ScrapedUrl)

					for _, url := range work.FoundUrls {
						foundHostname, err := getHostname(url)
						if err != nil {
							continue
						}

						if foundHostname == scrapedHostname {
							continue
						}

						foundHostnameAsUrl, err := getHostnameAsUrl(url)
						if err != nil {
							continue
						}

						// w.logger.Debug("Trying to add hostname connection", "from", scrapedHostname, "to", foundHostname, "foundHostnameAsUrl", foundHostnameAsUrl)
						if added := w.frontier.AddUrl(foundHostnameAsUrl); added {
							// w.logger.Debug("Adding hostname connection", "from", scrapedHostname, "to", foundHostname)
							w.frontend.AddHostnameConnection(scrapedHostname, foundHostname)
						}

					}

					w.frontend.NotifyEndCrawl(1, scrapedHostname, len(work.FoundUrls) == 0)
				}
			default:
				// TODO: Same sleep as the other? configurable?
				time.Sleep(time.Second)
			}
		}
	}()

	// Wait for either a stop message from the client or the duration to finish
	wsrecv := make(chan struct{})
	timer := time.NewTimer(cfg.Runtime.Duration)

	go func() {
		// Client can send anything and it will cancel the session.
		// Warning: As this is the thread-safe version, this will block any other reads.
		_, _, _ = ws.ReadMessage()
		// If we never read a message, the outer function call will return, closing the
		// WS and causing ReadMessage to return an error, which will end this goroutine.
		close(wsrecv)
	}()

	select {
	case <-timer.C:
	case <-wsrecv:
	}
}
