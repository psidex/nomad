package controller

import (
	"io"
	"log/slog"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/psidex/nomad/internal/frontier"
	"github.com/psidex/nomad/internal/lib"

	"github.com/psidex/nomad/internal/controller/pb"
)

var (
	// TODO: Remove, do this on the frontend
	DEBUG_TOTAL_METRICS = pb.ScrapeMetrics{}
)

type Server struct {
	pb.UnimplementedControllerServer
	//
	logger          *slog.Logger
	frontier        *frontier.Frontier
	stopper         *Stopper
	scrapedDataChan chan *pb.ScrapedData
	//
	Ws *WebServer
	// For setting worker IDs, not an actual count of workers currently connected
	workerIdCounter int32
}

func NewServer(logger *slog.Logger, randomCrawl bool) *Server {
	s := &Server{
		logger:          logger,
		frontier:        frontier.NewFrontier(randomCrawl),
		scrapedDataChan: make(chan *pb.ScrapedData),
		workerIdCounter: 0,
	}
	s.stopper = NewStopper(s.Flush)
	// Stop now - WebServer.Session will restart it
	s.stopper.Stop()
	ws := NewWebServer(s.logger, s.frontier, s.stopper, s.scrapedDataChan)
	s.Ws = ws
	return s
}

func (s *Server) Flush() {
	s.logger.Debug("Flushing frontier & scrapedDataChan")
	s.frontier.Flush()
	// Remove anything that finishing worker routines sent
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

	logger.Info("Registered new worker")

workerLoop:
	for {
		select {
		case <-ctx.Done():
			logger.Error("gRPC context is done", "err", ctx.Err().Error())
			return ctx.Err()
		default:
		}
		if url := s.frontier.PopUrl(); url != "" {
			if currentHostname, err := getHostname(url); err != nil {
				s.Ws.NotifyStartCrawl(workerId, currentHostname)
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
				break workerLoop
			}
			if err != nil {
				// TODO: Similar retry/break logic to the agent counterpart of this loop?
				logger.Error("Received error on worker stream", "workerId", workerId, "error", err)
				continue workerLoop
			}

			data := req.GetData()
			if data == nil {
				logger.Error("Received nil data from worker", "req", req)
			} else {
				DEBUG_TOTAL_METRICS.ResponseSizeBytes += data.Metrics.ResponseSizeBytes
				DEBUG_TOTAL_METRICS.NumFoundUrls += data.Metrics.NumFoundUrls
				DEBUG_TOTAL_METRICS.ScrapeDurationMs += data.Metrics.ScrapeDurationMs
				logger.Debug("", "DEBUG_TOTAL_METRICS", &DEBUG_TOTAL_METRICS)

				if s.stopper.IsStopped() {
					// Don't do anything with this data, continue operation as normal
					s.logger.Debug("Binning scrape data as stopper is stopped")
					continue workerLoop
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
