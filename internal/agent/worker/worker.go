package worker

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/psidex/nomad/internal/controller/pb"
	"github.com/psidex/nomad/internal/lib"
)

const (
	// TODO: Add these to config?

	// How many times the worker stream send/recv can error before abandoning
	streamErrCountThreshold = 5

	// How long to wait between trying to reconnect to the controller
	reconnectSleep = time.Second * 3
)

type Worker struct {
	// The worker ID as controlled by the controller
	id int32
	// The config provided by the controller for this worker
	cfg *pb.WorkerConfig
	// The global worker context
	ctx context.Context
	// The global master chromedp context
	chromedpCtx context.Context
	// The global worker waitgroup
	wg *sync.WaitGroup
	// This workers logger
	logger *slog.Logger
	// The controller client shared between all workers
	controller pb.ControllerClient
}

// New creates a new Worker struct
func New(ctx context.Context, parentLogger *slog.Logger, wg *sync.WaitGroup, chromedpCtx context.Context, number int, controller pb.ControllerClient) *Worker {
	// The number is our local "ID", just to identify logs from different goroutines
	logger := parentLogger.With("workerNumber", number)
	return &Worker{
		id:          0,
		cfg:         &pb.WorkerConfig{},
		ctx:         ctx,
		chromedpCtx: chromedpCtx,
		wg:          wg,
		logger:      logger,
		controller:  controller,
	}
}

// ReconnectLoop executes the Work function, and tries to reconnect (until the global
// worker context is cancelled) if disconnected from the controller unexpectedly
func (w Worker) ReconnectLoop() {
	defer w.wg.Done()

	w.logger.Info("Worker starting")

	for {
		if !w.Work() {
			break
		}
		w.logger.Info("Worker reconnecting", "waitDuration", reconnectSleep)
		select {
		case <-w.ctx.Done():
			// We check the ctx here as well as we could be in a recconnect loop
			w.logger.Info("Worker stopping due to context cancellation")
			return
		case <-time.After(reconnectSleep):
		}
	}

	w.logger.Info("Worker stopped")
}

// Work enters the worker stream with the controller.
// Returns true/false to indicate if it should be called again (for reconnecting)
func (w *Worker) Work() bool {
	w.logger.Info("Initiating worker stream with controller")
	stream, err := w.controller.WorkerStream(context.Background())
	if err != nil {
		w.logger.Error("Error creating worker stream", "error", err)
		return true
	}
	defer func() { _ = stream.CloseSend() }()

	w.logger.Info("Handshaking with controller")
	if err = stream.Send(
		&pb.WorkerMessage{
			Message: &pb.WorkerMessage_Handshake{
				Handshake: &pb.WorkerHandshake{
					NomadVersion: lib.NomadVersion,
				},
			},
		},
	); err != nil {
		w.logger.Error("Failed to handshake with controller", "error", err)
		// If the handshake failed, the fix probably wont be a simple reconnect
		return false
	}

	streamErrCount := 0

mainLoop:
	for {
		select {
		case <-w.ctx.Done():
			w.logger.Info("Stopping: Context cancelled")
			return false
		default:
		}

		if streamErrCount >= streamErrCountThreshold {
			w.logger.Warn("Stream error count threshold reached, abandoning connection", "count", streamErrCount)
			return true
		}

		var resp *pb.ControllerMessage
		resp, err = stream.Recv()
		if err == io.EOF || err != nil {
			w.logger.Error("Received error from worker stream", "error", err)
			streamErrCount++
			continue
		}

		switch msg := resp.Message.(type) {
		case *pb.ControllerMessage_ScrapeInstruction:
			for _, url := range msg.ScrapeInstruction.Urls {
				w.logger.Info("Scraping URL", "url", url)

				scrapedData := w.scrapeSinglePage(url)

				resp := &pb.WorkerMessage{
					Message: &pb.WorkerMessage_Data{
						Data: scrapedData,
					},
				}

				if err := stream.Send(resp); err != nil {
					w.logger.Error("Failed to send on worker stream", "error", err)
					streamErrCount++
					continue mainLoop
				}
			}

		case *pb.ControllerMessage_ConfigUpdate:
			w.logger.Info("Received worker config update", "config", msg.ConfigUpdate)
			w.id = msg.ConfigUpdate.WorkerId
			w.cfg = msg.ConfigUpdate

		case *pb.ControllerMessage_Shutdown:
			w.logger.Info("Stopping: Received shutdown from controller")
			return false

		default:
			w.logger.Error("Received unknown message type from controller", "message", resp)
			// We probably shouldn't try to reconnect if the controller is doing this
			return false
		}
	}
}

func (w Worker) scrapeSinglePage(urlToScrape string) *pb.ScrapedData {
	var data pb.ScrapedData

	switch w.cfg.Mode {
	case pb.WorkerMode_BASIC:
		data = w.makeBasicHttpRequest(urlToScrape)
	case pb.WorkerMode_CHROME:
		data = w.makeChromeRequest(urlToScrape)
	case pb.WorkerMode_HYBRID:
		data = w.makeBasicHttpRequest(urlToScrape)
		if data.Error != pb.ScrapeError_NONE || data.Metrics.NumFoundUrls <= 0 {
			w.logger.Info("Basic HTTP request failed, falling back to Chrome")
			data = w.makeChromeRequest(urlToScrape)
		}
	}

	if data.Error != pb.ScrapeError_NONE {
		return &pb.ScrapedData{
			AgentId:    w.id,
			ScrapedUrl: urlToScrape,
			FoundUrls:  []string{},
			Metrics:    &pb.ScrapeMetrics{},
			Error:      data.Error,
		}
	}

	return &data
}
