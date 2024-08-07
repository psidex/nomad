package main

import (
	"net"
	"net/http"
	"os"

	"log/slog"

	"google.golang.org/grpc"

	"github.com/psidex/nomad/internal/lib"

	"github.com/psidex/nomad/internal/controller"
	pb "github.com/psidex/nomad/internal/controller/pb"
)

const (
	// Default logging level, set using NOMAD_LOG_LEVEL
	defaultLogLevel = slog.LevelDebug
	// Default controller address, set using NOMAD_CONTROLLER_ADDRESS
	defaultControllerAddress = "nomad-controller:50051"
)

func main() {
	logLevel := defaultLogLevel
	if level := os.Getenv("NOMAD_LOG_LEVEL"); level != "" {
		var err error
		logLevel, err = lib.ParseSLogLevel(level)
		if err != nil {
			slog.Error("Invalid value for NOMAD_LOG_LEVEL", "value", level, "error", err)
			os.Exit(1)
		}
	}

	slog.SetLogLoggerLevel(logLevel)
	slog.Info("Starting controller")

	httpAddress := defaultControllerAddress
	if addr := os.Getenv("NOMAD_CONTROLLER_BIND_ADDRESS"); addr != "" {
		httpAddress = addr
	}

	slog.Info("Bind address configured", "address", httpAddress)

	lis, err := net.Listen("tcp", httpAddress)
	if err != nil {
		slog.Error("Failed to listen", "error", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer()

	nomadServer, err := controller.NewServer(false)
	if err != nil {
		slog.Error("Failed to create new gRPC server", "error", err)
		os.Exit(1)
	}

	pb.RegisterControllerServer(grpcServer, nomadServer)

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			slog.Error("Failed to serve gRPC", "error", err)
			os.Exit(1)
		}
	}()

	//
	// http stuff
	//

	staticDir := "public"
	httpAddress2 := "0.0.0.0:8080"

	http.Handle("/", http.FileServer(http.Dir(staticDir)))
	http.HandleFunc("/ws", nomadServer.Session)

	if err := http.ListenAndServe(httpAddress2, nil); err != nil {
		slog.Error("Failed to serve HTTP", "error", err)
		os.Exit(1)
	}

	slog.Info("Gracefully finished, goodbye")
}
