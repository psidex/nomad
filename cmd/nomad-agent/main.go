package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/chromedp/chromedp"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/psidex/nomad/internal/controller/pb"
	"github.com/psidex/nomad/internal/lib"

	"github.com/psidex/nomad/internal/agent/config"
	"github.com/psidex/nomad/internal/agent/worker"
)

func main() {
	cfg, err := config.Init()
	if err != nil {
		slog.Error("Could not load config", "error", err)
		os.Exit(1)
	}

	logger := lib.NiceLogger(os.Stdout, cfg.LogLevel)
	defer logger.Info("Agent stopped")
	logger.Info(
		"Starting nomad-agent",
		"version", lib.NomadVersion,
		"commit", lib.GitCommit[0:7]+lib.GitDirty,
		"commitTime", lib.GitTime,
		"config", fmt.Sprintf("%+v", cfg),
	)

	logger.Info("Creating controller client")

	conn, err := grpc.NewClient(
		cfg.ControllerAddress,
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
	if cfg.DebugChromeGUI {
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

	logger.Info("Configured worker count", "count", cfg.WorkerCount)

	workersCtx, stopWorkers := context.WithCancel(context.Background())

	workerWg := &sync.WaitGroup{}
	workerWg.Add(cfg.WorkerCount)

	logger.Info("Spinning up workers")
	for i := 0; i < cfg.WorkerCount; i++ {
		go worker.New(
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
		// will then pickup the stopped ctx
		stopWorkers()
		conn.Close()
		workerWg.Wait()
	case <-wgFinishedChan:
		logger.Info("All workers stopped, shutting down")
	}

	// defers
}
