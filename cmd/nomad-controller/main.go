package main

import (
	"log/slog"
	"net"
	"net/http"
	"os"

	"github.com/charmbracelet/log"
	"google.golang.org/grpc"

	"github.com/psidex/nomad/internal/lib"

	"github.com/psidex/nomad/internal/controller"
	pb "github.com/psidex/nomad/internal/controller/pb"
)

const (
	// Default logging level, set using NOMAD_LOG_LEVEL
	defaultLogLevel = log.DebugLevel
	// Default controller address, set using NOMAD_CONTROLLER_GRPC_ADDRESS
	defaultControllerAddress = "0.0.0.0:50051"
	// Default HTTP server address, set using NOMAD_CONTROLLER_HTTP_ADDRESS
	defaultHttpAddress = "0.0.0.0:8080"
)

func initGrpc(logger *slog.Logger) *controller.Server {
	grpcBindAddr := defaultControllerAddress
	if addr := os.Getenv("NOMAD_CONTROLLER_GRPC_ADDRESS"); addr != "" {
		grpcBindAddr = addr
	}

	logger.Info("gRPC listen address configured", "address", grpcBindAddr)

	lis, err := net.Listen("tcp", grpcBindAddr)
	if err != nil {
		logger.Error("Failed to listen", "error", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer()

	controllerGrpcServer, err := controller.NewServer(logger, false)
	if err != nil {
		logger.Error("Failed to create new gRPC server", "error", err)
		os.Exit(1)
	}

	pb.RegisterControllerServer(grpcServer, controllerGrpcServer)

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			logger.Error("Failed to serve gRPC", "error", err)
			os.Exit(1)
		}
	}()

	return controllerGrpcServer
}

func initHttp(logger *slog.Logger, controllerGrpcServer *controller.Server) {
	httpBindAddress := defaultHttpAddress
	if addr := os.Getenv("NOMAD_CONTROLLER_HTTP_ADDRESS"); addr != "" {
		httpBindAddress = addr
	}

	logger.Info("HTTP listen address configured", "address", httpBindAddress)

	staticDir := "public"

	http.Handle("/", http.FileServer(http.Dir(staticDir)))
	http.HandleFunc("/ws", controllerGrpcServer.Session)

	if err := http.ListenAndServe(httpBindAddress, nil); err != nil {
		logger.Error("Failed to serve HTTP", "error", err)
		os.Exit(1)
	}
}

func main() {
	logLevel := defaultLogLevel
	if level := os.Getenv("NOMAD_LOG_LEVEL"); level != "" {
		var err error
		if logLevel, err = log.ParseLevel(level); err != nil {
			slog.Error("Invalid value for NOMAD_LOG_LEVEL", "value", level, "error", err)
			os.Exit(1)
		}
	}

	logger := lib.NiceLogger(os.Stdout, logLevel)
	logger.Info(
		"Starting nomad-controller",
		"version", lib.NomadVersion,
		"commit", lib.GitCommit[0:7]+lib.GitDirty,
		"commitTime", lib.GitTime,
	)

	controllerGrpcServer := initGrpc(logger)

	// Will block until we want to exit / crash
	initHttp(logger, controllerGrpcServer)

	logger.Info("Stopped, goodbye")
}
