package main

import (
	"context"
	"io"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/psidex/nomad/internal/agent"
	pb "github.com/psidex/nomad/internal/controller/pb"
	"github.com/psidex/nomad/internal/lib"
)

const (
	// Should always match the controller version
	nomadVersion int64 = 0

	// How long to wait between trying to reconnect to the controller
	reconnectSleep = time.Second * 3

	// How many times the worker stream send/recv can error before abandoning
	streamErrCountThreshold = 5

	// Default logging level, set using NOMAD_LOG_LEVEL
	defaultLogLevel = slog.LevelDebug
	// Default controller address, set using NOMAD_CONTROLLER_ADDRESS
	defaultControllerAddress = "nomad-controller:50051"
	// Default worker count, set using NOMAD_AGENT_WORKER_COUNT
	defaultWorkerCount = 1
)

// worker returns true/false to indicate if it should be called again (for reconnecting)
func worker(ctx context.Context, addr string) bool {
	slog.Info("Connecting to controller", "address", addr)

	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		slog.Error("Could not connect to controller", "error", err)
		return true
	}
	defer func() { _ = conn.Close() }()

	controller := pb.NewControllerClient(conn)
	worker := agent.Worker{}

	slog.Info("Initiating worker stream with controller")
	stream, err := controller.WorkerStream(context.Background())
	if err != nil {
		slog.Error("Error creating worker stream", "error", err)
		return true
	}
	defer func() { _ = stream.CloseSend() }()

	slog.Info("Handshaking with controller")
	if err = stream.Send(
		&pb.WorkerMessage{
			Message: &pb.WorkerMessage_Handshake{
				Handshake: &pb.WorkerHandshake{
					NomadVersion: nomadVersion,
				},
			},
		},
	); err != nil {
		slog.Error("Failed to handshake with controller", "error", err)
		// If the handshake failed, the fix probably wont be a simple reconnect
		return false
	}

	streamErrCount := 0

mainLoop:
	for {
		select {
		case <-ctx.Done():
			slog.Info("Stopping: Context cancelled")
			return false
		default:
		}

		if streamErrCount >= streamErrCountThreshold {
			slog.Warn("Stream error count threshold reached, abandoning connection", "streamErrCount", streamErrCount)
			return true
		}

		var resp *pb.ControllerMessage
		resp, err = stream.Recv()
		if err == io.EOF || err != nil {
			slog.Error("Received error from worker stream", "error", err)
			streamErrCount++
			continue
		}

		switch msg := resp.Message.(type) {
		case *pb.ControllerMessage_ScrapeInstruction:
			for _, url := range msg.ScrapeInstruction.Urls {
				slog.Info("Scraping URL", "url", url)

				scrapedData := worker.ScrapeSinglePage(url)
				resp := &pb.WorkerMessage{
					Message: &pb.WorkerMessage_Data{
						Data: scrapedData,
					},
				}

				if err := stream.Send(resp); err != nil {
					slog.Error("Failed to send on worker stream", "error", err)
					streamErrCount++
					continue mainLoop
				}
			}

		case *pb.ControllerMessage_ConfigUpdate:
			slog.Info("Received worker config update", "config", msg.ConfigUpdate)
			worker.Id = msg.ConfigUpdate.WorkerId
			worker.Cfg = msg.ConfigUpdate

		case *pb.ControllerMessage_Shutdown:
			slog.Info("Stopping: Received shutdown from controller")
			return false

		default:
			slog.Error("Received unknown message type from controller", "message", resp)
			// We probably shouldn't try to reconnect if the controller is doing this
			return false
		}
	}
}

func workerReconnectLoop(ctx context.Context, wg *sync.WaitGroup, controllerAddress string) {
	defer wg.Done()
	slog.Info("Worker starting")
	for {
		if !worker(ctx, controllerAddress) {
			break
		}
		slog.Info("Worker reconnecting", "waitDuration", reconnectSleep)
		select {
		case <-ctx.Done():
			// We check the ctx here as well as we could be in a recconnect loop
			slog.Info("Worker stopping due to context cancellation")
			return
		case <-time.After(reconnectSleep):
		}
	}
	slog.Info("Worker stopped")
}

func main() {
	logLevel := defaultLogLevel
	if level := os.Getenv("NOMAD_LOG_LEVEL"); level != "" {
		var err error
		logLevel, err = lib.ParseSLogLevel(level)
		if err != nil {
			slog.Error("Invalid value for NOMAD_LOG_LEVEL", "value", level, "error", err)
			return
		}
	}

	slog.SetLogLoggerLevel(logLevel)
	slog.Info("Agent starting")

	controllerAddress := defaultControllerAddress
	if addr := os.Getenv("NOMAD_CONTROLLER_ADDRESS"); addr != "" {
		controllerAddress = addr
	}

	workerCount := defaultWorkerCount
	if count := os.Getenv("NOMAD_AGENT_WORKER_COUNT"); count != "" {
		var err error
		workerCount, err = strconv.Atoi(count)
		if err != nil || workerCount <= 0 {
			slog.Error("Invalid value for NOMAD_AGENT_WORKER_COUNT", "value", count, "error", err)
			return
		}
	}

	slog.Info("Configured worker count", "count", workerCount)

	workersCtx, stopWorkers := context.WithCancel(context.Background())

	wg := &sync.WaitGroup{}
	wg.Add(workerCount)

	for i := 0; i < workerCount; i++ {
		go workerReconnectLoop(workersCtx, wg, controllerAddress)
	}

	wgFinishedChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(wgFinishedChan)
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigChan:
		slog.Info("Process received SIGINT/SIGTERM, shutting down")
		stopWorkers()
		wg.Wait()
	case <-wgFinishedChan:
		slog.Info("All workers stopped, shutting down")
	}

	slog.Info("Agent stopped")
}
