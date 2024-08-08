package controller

import (
	"encoding/json"
	"io"
	"log"
	"log/slog"
	"net/http"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

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
	Runtime           lib.Duration `json:"runtime"`           // unused
	HttpClientTimeout lib.Duration `json:"httpClientTimeout"` // unused
	WorkerCooldown    lib.Duration `json:"workerCooldown"`    // unused
	WorkerCount       uint         `json:"workerCount"`       // unused
	InitialUrls       []string     `json:"initialUrls"`
	RandomCrawl       bool         `json:"randomCrawl"`
}

type Server struct {
	pb.UnimplementedControllerServer
	logger *slog.Logger
	// TODO: urlsToScrape batching using []string
	urlsToScrape chan string
	outputs      chan *pb.ScrapedData
	frontEnd     *graphology.GraphologyWs
	// For setting worker IDs // TODO: -1 when worker drops?
	workerCount int32
	// Set / reset per run
	frontier frontier.Frontier
}

func init() {
	upgrader.CheckOrigin = func(r *http.Request) bool { return true }
}

func NewServer(logger *slog.Logger, randomCrawl bool) (*Server, error) {
	s := &Server{
		logger:       logger,
		urlsToScrape: make(chan string),
		outputs:      make(chan *pb.ScrapedData),
		workerCount:  0,
		frontier:     frontier.NewFrontier(randomCrawl),
	}

	go s.feedWorkers()

	return s, nil
}

func (s *Server) Session(w http.ResponseWriter, r *http.Request) {
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

	s.frontEnd = graphology.NewGraphologyWs(ws)

	for _, initialUrl := range cfg.InitialUrls {
		toAdd, err := getHostnameAsUrl(initialUrl)
		if err != nil {
			log.Println("getHostnameAsUrl err:", err)
			return
		}
		s.frontier.AddUrl(toAdd)
	}

	for {
		work := <-s.outputs
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

				if added := s.frontier.AddUrl(foundHostnameAsUrl); added {
					s.frontEnd.AddHostnameConnection(scrapedHostname, foundHostname)
				}

			}

			// TODO: dead-end support
			s.frontEnd.NotifyEndCrawl(1, scrapedHostname, false)
		}
	}
}

func (s *Server) feedWorkers() {
	// TODO: Cancellation! and general graceful shutdown
	for {
		if currentlUrl := s.frontier.PopUrl(); currentlUrl != "" {
			// Unbuffered chan means we just sit here until something can read
			s.urlsToScrape <- currentlUrl

			//
			if currentHostname, err := getHostname(currentlUrl); err == nil {
				if s.frontEnd != nil {
					s.frontEnd.NotifyStartCrawl(1, currentHostname)
				}
			}

		} else {
			// Don't spam popurl as fast as possible
			// TODO: Config time here?
			time.Sleep(time.Second)
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

	s.workerCount += 1
	workerId := s.workerCount

	// Send configuration to the worker
	configUpdate := &pb.WorkerConfig{
		WorkerId:              workerId,
		SingleScrapeTimeoutMs: 100_000,
	}
	err = srv.Send(&pb.ControllerMessage{
		Message: &pb.ControllerMessage_ConfigUpdate{
			ConfigUpdate: configUpdate,
		},
	})
	if err != nil {
		s.logger.Error("Failed to send worker config", "workerId", workerId, "error", err)
		return nil
	}

	// Work loop
	for {
		select {
		case <-ctx.Done():
			s.logger.Error("gRPC context is done", "err", ctx.Err().Error())
			return ctx.Err()
		default:
		}

		url := <-s.urlsToScrape
		s.logger.Debug("Issuing URL to worker", "url", url)

		resp := pb.ControllerMessage{
			Message: &pb.ControllerMessage_ScrapeInstruction{
				ScrapeInstruction: &pb.ScrapeInstruction{Urls: []string{url}},
			},
		}
		if err := srv.Send(&resp); err != nil {
			s.logger.Error("Failed to send on worker stream", "workerId", workerId, "error", err)
		}

		req, err := srv.Recv()
		if err == io.EOF {
			s.logger.Error("Received EOF on worker stream", "workerId", workerId)
			break
		}
		if err != nil {
			s.logger.Error("Received error on worker stream", "workerId", workerId, "error", err)
			continue
		}

		s.logger.Debug("Scrape loop end", "workerId", workerId, "request", req)
		data := req.GetData()
		if data == nil {
			s.logger.Error("Received nil data from worker", "req", req)
		} else {
			s.outputs <- data
		}
	}

	if err := srv.Send(&pb.ControllerMessage{
		Message: &pb.ControllerMessage_Shutdown{},
	}); err != nil {
		s.logger.Error("Failed to send shutdown message", "workerId", workerId, "error", err)
	}

	s.logger.Debug("Scrape function end", "workerId", workerId)
	return nil
}
