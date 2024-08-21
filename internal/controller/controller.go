package controller

import (
	"encoding/json"
	"io"
	"log"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/psidex/nomad/internal/frontier"
	"github.com/psidex/nomad/internal/graphology"
	"github.com/psidex/nomad/internal/lib"

	"github.com/psidex/nomad/internal/controller/pb"
)

// TODO: Tidy this up! probably refactor needed

var (
	upgrader                = websocket.Upgrader{}
	DEBUG_TOTAL_BYTES int64 = 0
)

type SessionConfig struct {
	Runtime           lib.Duration `json:"runtime"`           // unused
	HttpClientTimeout lib.Duration `json:"httpClientTimeout"` // unused
	WorkerCooldown    lib.Duration `json:"workerCooldown"`    // unused
	WorkerCount       uint         `json:"workerCount"`       // unused
	RandomCrawl       bool         `json:"randomCrawl"`       // unused
	InitialUrls       []string     `json:"initialUrls"`
}

type Server struct {
	pb.UnimplementedControllerServer
	logger          *slog.Logger
	stopScraping    chan struct{}
	scrapedDataChan chan *pb.ScrapedData
	frontEnd        *graphology.GraphologyWs
	// For setting worker IDs, not an actual count of workers currently connected
	workerIdCounter int32
	// Set / reset per run
	frontier *frontier.Frontier
}

func init() {
	upgrader.CheckOrigin = func(r *http.Request) bool { return true }
}

func NewServer(logger *slog.Logger, randomCrawl bool) (*Server, error) {
	return &Server{
		logger:          logger,
		stopScraping:    make(chan struct{}),
		scrapedDataChan: make(chan *pb.ScrapedData),
		workerIdCounter: 0,
		frontier:        frontier.NewFrontier(randomCrawl),
	}, nil
}

func (s *Server) Session(w http.ResponseWriter, r *http.Request) {
	// TODO: Only allow one session at a time, e.g. check here and then reject

	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("ws upgrade err:", err)
		return
	}
	defer c.Close()

	ws := lib.NewThreadSafeWebSocket(c)

	_, msg, err := ws.ReadMessage()
	if err != nil {
		log.Println("ws cfg read err:", err)
		return
	}

	cfg := &SessionConfig{}
	if err = json.Unmarshal(msg, cfg); err != nil {
		log.Println("ws cfg unmarshal err:", err)
		return
	}

	s.stopScraping = make(chan struct{})
	s.frontEnd = graphology.NewGraphologyWs(ws)

	for _, initialUrl := range cfg.InitialUrls {
		toAdd, err := getHostnameAsUrl(initialUrl)
		if err != nil {
			log.Println("getHostnameAsUrl err:", err)
			return
		}
		s.frontier.AddUrl(toAdd)
	}

	// Take scraped data, process it, send info to front end if required
	go func() {
		for {
			select {
			case <-s.stopScraping:
				return
			case work := <-s.scrapedDataChan:
				s.logger.Debug("handle work loop got work", "work", work)
				if s.frontEnd != nil {
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

						s.logger.Debug("Trying to add hostname connection", "from", scrapedHostname, "to", foundHostname, "foundHostnameAsUrl", foundHostnameAsUrl)
						if added := s.frontier.AddUrl(foundHostnameAsUrl); added {
							s.logger.Debug("Adding hostname connection", "from", scrapedHostname, "to", foundHostname)
							s.frontEnd.AddHostnameConnection(scrapedHostname, foundHostname)
						}

					}
					// TODO: dead-end support
					s.frontEnd.NotifyEndCrawl(1, scrapedHostname, false)
				}
			default:
				// TODO: Same sleep as the other?
				time.Sleep(time.Second)
			}
		}
	}()

	// Wait for either a stop messgae from the client of the duration to finish
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

	s.cancelCurrent()
}

func (s *Server) cancelCurrent() {
	close(s.stopScraping)
	s.frontier.Flush()
	// Empty the channel just in case
L:
	for {
		select {
		case <-s.scrapedDataChan:
		default:
			break L
		}
	}
}

func (s *Server) WorkerStream(srv pb.Controller_WorkerStreamServer) error {
	ctx := srv.Context()

	// recv handshake
	workerMessage, err := srv.Recv()
	if err == io.EOF {
		s.logger.Error("Received EOF on unknown worker stream")
		return nil
	}
	if err != nil {
		s.logger.Error("Received error on unknown worker stream", "error", err)
		return nil
	}

	// Check if the received message is a handshake
	handshake := workerMessage.GetHandshake()
	if handshake == nil {
		s.logger.Error("Did not receive expected worker handshake message", "workerMessage", workerMessage)
		return nil
	}

	// Validate the handshake
	if handshake.NomadVersion != lib.NomadVersion {
		return status.Errorf(
			codes.PermissionDenied,
			"version mismatch: expected %d, got %d",
			lib.NomadVersion, handshake.NomadVersion,
		)
	}

	s.workerIdCounter += 1
	workerId := s.workerIdCounter

	logger := s.logger.With("workerId", workerId)

	// Send configuration to the worker
	configUpdate := &pb.WorkerConfig{
		WorkerId: workerId,
		// TODO: These should come from the frontend
		SingleScrapeTimeoutMs: 5000,
		Mode:                  pb.WorkerMode_HYBRID,
	}
	err = srv.Send(&pb.ControllerMessage{
		Message: &pb.ControllerMessage_ConfigUpdate{
			ConfigUpdate: configUpdate,
		},
	})
	if err != nil {
		logger.Error("Failed to send worker config", "workerId", workerId, "error", err)
		return nil
	}

workLoop:
	for {
		select {
		case <-ctx.Done():
			logger.Error("gRPC context is done", "err", ctx.Err().Error())
			return ctx.Err()
		default:
		}
		if url := s.frontier.PopUrl(); url != "" {
			if currentHostname, err := getHostname(url); err != nil && s.frontEnd != nil {
				s.frontEnd.NotifyStartCrawl(uint(workerId), currentHostname)
			}

			logger.Debug("Issuing URL to worker", "url", url)

			resp := pb.ControllerMessage{
				Message: &pb.ControllerMessage_ScrapeInstruction{
					ScrapeInstruction: &pb.ScrapeInstruction{Urls: []string{url}},
				},
			}
			if err := srv.Send(&resp); err != nil {
				logger.Error("Failed to send on worker stream", "workerId", workerId, "error", err)
			}

			req, err := srv.Recv()
			if err == io.EOF {
				logger.Error("Received EOF on worker stream", "workerId", workerId)
				break workLoop
			}
			if err != nil {
				// TODO: Similar retry/break logic to the agent counterpart of this loop?
				logger.Error("Received error on worker stream", "workerId", workerId, "error", err)
				continue workLoop
			}

			data := req.GetData()
			if data == nil {
				logger.Error("Received nil data from worker", "req", req)
			} else {
				// TODO: Remove this global var and log?
				DEBUG_TOTAL_BYTES += data.Metrics.ResponseSizeBytes
				megabytes := float64(DEBUG_TOTAL_BYTES) / (1024 * 1024)
				logger.Debug("Downloaded", "megabytes", megabytes)
				select {
				case <-s.stopScraping:
					// Don't do anything with this data, continue operation as normal
					continue workLoop
				default:
				}
				s.scrapedDataChan <- data
			}
		} else {
			// TODO: Same sleep as the other? configurable?
			time.Sleep(time.Second)
		}
	}

	if err := srv.Send(&pb.ControllerMessage{
		Message: &pb.ControllerMessage_Shutdown{},
	}); err != nil {
		logger.Error("Failed to send shutdown message", "workerId", workerId, "error", err)
	}

	logger.Debug("Scrape function end", "workerId", workerId)
	return nil
}
