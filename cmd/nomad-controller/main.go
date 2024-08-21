package main

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"

	"google.golang.org/grpc"

	"github.com/psidex/nomad/internal/lib"

	"github.com/psidex/nomad/internal/controller"
	"github.com/psidex/nomad/internal/controller/config"
	pb "github.com/psidex/nomad/internal/controller/pb"
)

// TODO: A dead worker causes scrape to not start

func initGrpc(logger *slog.Logger, cfg config.Config) *controller.Server {
	logger.Info("gRPC listen address configured", "address", cfg.GrpcAddress)

	lis, err := net.Listen("tcp", cfg.GrpcAddress)
	if err != nil {
		logger.Error("Failed to listen", "error", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer()
	controllerGrpcServer := controller.NewServer(logger, false)
	pb.RegisterControllerServer(grpcServer, controllerGrpcServer)

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			logger.Error("Failed to serve gRPC", "error", err)
			os.Exit(1)
		}
	}()

	return controllerGrpcServer
}

func initHttp(logger *slog.Logger, cfg config.Config, controller *controller.Server) {
	logger.Info("HTTP listen address configured", "address", cfg.HttpAddress)

	staticDir := "public"

	http.Handle("/", http.FileServer(http.Dir(staticDir)))
	http.HandleFunc("/ws", controller.Ws.Session)

	if err := http.ListenAndServe(cfg.HttpAddress, nil); err != nil {
		logger.Error("Failed to serve HTTP", "error", err)
		os.Exit(1)
	}
}

func main() {
	cfg, err := config.Init()
	if err != nil {
		slog.Error("Could not load config", "error", err)
		os.Exit(1)
	}

	logger := lib.NiceLogger(os.Stdout, cfg.LogLevel)
	logger.Info(
		"Starting nomad-controller",
		"version", lib.NomadVersion,
		"commit", lib.GitCommit[0:7]+lib.GitDirty,
		"commitTime", lib.GitTime,
		"config", fmt.Sprintf("%+v", cfg),
	)

	controllerGrpcServer := initGrpc(logger, cfg)

	// Will block until we want to exit / crash
	initHttp(logger, cfg, controllerGrpcServer)

	logger.Info("Stopped, goodbye")
}
