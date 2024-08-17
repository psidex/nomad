package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/log"

	"github.com/psidex/nomad/internal/agent"
	"github.com/psidex/nomad/internal/lib"
)

const (
	// How long to wait between trying to reconnect to the controller
	reconnectSleep = time.Second * 3

	// Default logging level, set using NOMAD_LOG_LEVEL
	defaultLogLevel = log.DebugLevel
	// Default controller address, set using NOMAD_CONTROLLER_GRPC_ADDRESS
	defaultControllerAddress = "nomad-controller:50051"
	// Default worker count, set using NOMAD_AGENT_WORKER_COUNT
	defaultWorkerCount = 1
)

func workerReconnectLoop(ctx context.Context, logger *slog.Logger, wg *sync.WaitGroup, controllerAddress string) {
	defer wg.Done()
	logger.Info("Worker starting")

	w := agent.NewWorker(ctx, logger, controllerAddress)
	for {
		if !w.Work() {
			break
		}
		logger.Info("Worker reconnecting", "waitDuration", reconnectSleep)
		select {
		case <-ctx.Done():
			// We check the ctx here as well as we could be in a recconnect loop
			logger.Info("Worker stopping due to context cancellation")
			return
		case <-time.After(reconnectSleep):
		}
	}

	logger.Info("Worker stopped")
}

func main() {
	logLevel := defaultLogLevel
	if level := os.Getenv("NOMAD_LOG_LEVEL"); level != "" {
		var err error
		logLevel, err = log.ParseLevel(level)
		if err != nil {
			slog.Error("Invalid value for NOMAD_LOG_LEVEL", "value", level, "error", err)
			return
		}
	}

	logger := lib.NiceLogger(os.Stdout, logLevel)
	logger.Info("Starting nomad-agent", "version", lib.NomadVersion, "commit", lib.GitCommit[0:7]+lib.GitDirty, "time", lib.GitTime)

	controllerAddress := defaultControllerAddress
	if addr := os.Getenv("NOMAD_CONTROLLER_GRPC_ADDRESS"); addr != "" {
		controllerAddress = addr
	}

	workerCount := defaultWorkerCount
	if count := os.Getenv("NOMAD_AGENT_WORKER_COUNT"); count != "" {
		var err error
		workerCount, err = strconv.Atoi(count)
		if err != nil || workerCount <= 0 {
			logger.Error("Invalid value for NOMAD_AGENT_WORKER_COUNT", "value", count, "error", err)
			return
		}
	}

	logger.Info("Configured worker count", "count", workerCount)

	workersCtx, stopWorkers := context.WithCancel(context.Background())

	wg := &sync.WaitGroup{}
	wg.Add(workerCount)

	for i := 0; i < workerCount; i++ {
		go workerReconnectLoop(workersCtx, logger, wg, controllerAddress)
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
		logger.Info("Process received SIGINT/SIGTERM, shutting down")
		stopWorkers()
		wg.Wait()
	case <-wgFinishedChan:
		logger.Info("All workers stopped, shutting down")
	}

	logger.Info("Agent stopped")
}
