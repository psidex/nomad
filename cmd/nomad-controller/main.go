package main

import (
	"io"
	"net"
	"os"

	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/psidex/nomad/internal/controller/pb"
	"github.com/psidex/nomad/internal/lib"
)

const (
	// Should always match the controller version
	nomadVersion int64 = 0

	// Default logging level, set using NOMAD_LOG_LEVEL
	defaultLogLevel = slog.LevelDebug
	// Default controller address, set using NOMAD_CONTROLLER_ADDRESS
	defaultControllerAddress = "nomad-controller:50051"
)

type server struct {
	pb.UnimplementedControllerServer
	agentCount int32
}

func (s *server) WorkerStream(srv pb.Controller_WorkerStreamServer) error {
	ctx := srv.Context()

	// recv handshake
	workerMessage, err := srv.Recv()
	if err == io.EOF {
		slog.Error("Received EOF on unknown worker stream")
		return nil
	}
	if err != nil {
		slog.Error("Received error on unknown worker stream", "error", err)
		return nil
	}

	// Check if the received message is a handshake
	handshake := workerMessage.GetHandshake()
	if handshake == nil {
		slog.Error("Did not receive expected worker handshake message", "workerMessage", workerMessage)
		return nil
	}

	// Validate the handshake
	if handshake.NomadVersion != nomadVersion {
		return status.Errorf(
			codes.PermissionDenied,
			"version mismatch: expected %d, got %d",
			nomadVersion, handshake.NomadVersion,
		)
	}

	s.agentCount += 1
	workerId := s.agentCount

	// Send configuration to the worker
	configUpdate := &pb.WorkerConfig{
		WorkerId:              workerId,
		SingleScrapeTimeoutMs: 5000,
	}
	err = srv.Send(&pb.ControllerMessage{
		Message: &pb.ControllerMessage_ConfigUpdate{
			ConfigUpdate: configUpdate,
		},
	})
	if err != nil {
		slog.Error("Failed to send worker config", "workerId", workerId, "error", err)
		return nil
	}

	// Work loop
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		resp := pb.ControllerMessage{
			Message: &pb.ControllerMessage_ScrapeInstruction{
				ScrapeInstruction: &pb.ScrapeInstruction{Urls: []string{"https://pkg.go.dev/time"}},
			},
		}
		if err := srv.Send(&resp); err != nil {
			slog.Error("Failed to send on worker stream", "workerId", workerId, "error", err)
		}

		req, err := srv.Recv()
		if err == io.EOF {
			slog.Error("Received EOF on worker stream", "workerId", workerId)
			break
		}
		if err != nil {
			slog.Error("Received error on worker stream", "workerId", workerId, "error", err)
			continue
		}

		slog.Debug("Scrape loop end", "workerId", workerId, "request", req)
		break
	}

	if err := srv.Send(&pb.ControllerMessage{
		Message: &pb.ControllerMessage_Shutdown{},
	}); err != nil {
		slog.Error("Failed to send shutdown message", "workerId", workerId, "error", err)
	}

	slog.Debug("Scrape function end", "workerId", workerId)
	return nil
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
	slog.Info("Starting controller")

	address := defaultControllerAddress
	if addr := os.Getenv("NOMAD_CONTROLLER_BIND_ADDRESS"); addr != "" {
		address = addr
	}

	slog.Info("Bind address configured", "address", address)

	lis, err := net.Listen("tcp", address)
	if err != nil {
		slog.Error("Failed to listen", "error", err)
		return
	}

	s := grpc.NewServer()

	pb.RegisterControllerServer(s, &server{})

	if err := s.Serve(lis); err != nil {
		slog.Error("Failed to serve", "error", err)
	}
}
