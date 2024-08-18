package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"

	"github.com/charmbracelet/log"
	"github.com/chromedp/chromedp"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/psidex/nomad/internal/controller/pb"
	"github.com/psidex/nomad/internal/lib"

	"github.com/psidex/nomad/internal/agent"
)

const (
	// Default logging level, set using NOMAD_LOG_LEVEL
	defaultLogLevel = log.DebugLevel
	// Default controller address, set using NOMAD_CONTROLLER_GRPC_ADDRESS
	defaultControllerAddress = "nomad-controller:50051"
	// Default worker count, set using NOMAD_AGENT_WORKER_COUNT
	defaultWorkerCount = 1

	// Debugging option to show Chrome GUI, obviously will only work if you're executing
	// in a GUI environment
	debugVisualChrome = false
)

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
	defer logger.Info("Agent stopped")
	logger.Info(
		"Starting nomad-agent",
		"version", lib.NomadVersion,
		"commit", lib.GitCommit[0:7]+lib.GitDirty,
		"commitTime", lib.GitTime,
	)

	logger.Info("Creating controller client")

	controllerAddress := defaultControllerAddress
	if addr := os.Getenv("NOMAD_CONTROLLER_GRPC_ADDRESS"); addr != "" {
		controllerAddress = addr
	}

	conn, err := grpc.NewClient(
		controllerAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		logger.Error("Could create controller client", "error", err)
		os.Exit(1)
	}
	defer func() { _ = conn.Close() }()

	// Share one client between all workers, this is documented as safe and as far as I
	// can tell, shouldn't cause any performance issues.
	// https://github.com/grpc/grpc-go/blob/master/Documentation/concurrency.md
	controller := pb.NewControllerClient(conn)

	logger.Info("Warming up Chrome")

	baseChromeCtx := context.Background()
	if debugVisualChrome {
		// https://github.com/chromedp/chromedp/issues/311#issuecomment-1029950927
		var cancelBase context.CancelFunc
		baseChromeCtx, cancelBase = chromedp.NewExecAllocator(
			context.Background(),
			append(
				chromedp.DefaultExecAllocatorOptions[:],
				chromedp.Flag("headless", false),
			)...,
		)
		defer cancelBase()
	}

	// Create a master chromedp context which when run will start the main browser
	// process and allow us to create new tabs within it
	chromedpCtx, cancel := chromedp.NewContext(baseChromeCtx)
	defer cancel()

	// Ensure the browser process is running
	if err := chromedp.Run(chromedpCtx); err != nil {
		logger.Error("Chrome warmup failed, continuing", "error", err)
	}

	workerCount := defaultWorkerCount
	if count := os.Getenv("NOMAD_AGENT_WORKER_COUNT"); count != "" {
		var err error
		workerCount, err = strconv.Atoi(count)
		if err != nil || workerCount <= 0 {
			logger.Error("Invalid value for NOMAD_AGENT_WORKER_COUNT", "value", count, "error", err)
			os.Exit(1)
		}
	}

	logger.Info("Configured worker count", "count", workerCount)

	workersCtx, stopWorkers := context.WithCancel(context.Background())

	workerWg := &sync.WaitGroup{}
	workerWg.Add(workerCount)

	logger.Info("Spinning up workers")
	for i := 0; i < workerCount; i++ {
		go agent.NewWorker(
			workersCtx, logger, workerWg, chromedpCtx, i, controller,
		).ReconnectLoop()
	}

	wgFinishedChan := make(chan struct{})
	go func() {
		workerWg.Wait()
		close(wgFinishedChan)
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	logger.Info("Main thread waiting for SIGINT / SIGTERM / WaitGroup")

	select {
	case <-sigChan:
		logger.Info("Process received SIGINT/SIGTERM, shutting down")
		// Cancel worker context and then close controller conn, if any workers are
		// waiting to Recv(), the closing of conn will trigger another work loop which
		// will then pickup the ctx
		stopWorkers()
		conn.Close()
		workerWg.Wait()
	case <-wgFinishedChan:
		logger.Info("All workers stopped, shutting down")
	}

	// defers
}
