package main

import (
	"io"
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/psidex/nomad/internal/controller/pb"
)

const (
	nomadVersion = 0
	address      = "0.0.0.0:50051"
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
		log.Println("Received EOF on unknown worker stream")
		return nil
	}
	if err != nil {
		log.Printf("Received error on unknown worker stream: %s", err)
		return nil
	}

	// Check if the received message is a handshake
	handshake := workerMessage.GetHandshake()
	if handshake == nil {
		log.Println("Expected worker handshake message")
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
		log.Printf("[%d] Failed to send worker config: %s", workerId, err)
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
			log.Printf("[%d] Failed to send on worker stream: %s", workerId, err)
		}

		req, err := srv.Recv()
		if err == io.EOF {
			log.Printf("[%d] Received EOF on worker stream", workerId)
			break
		}
		if err != nil {
			log.Printf("[%d] Received error on worker stream: %s", workerId, err)
			continue
		}

		log.Printf("[%d] Debug: scrape loop end, recvd: %+v", workerId, req)
		break
	}

	log.Printf("[%d] Debug: scrape function end", workerId)
	return nil
}

func main() {
	log.Printf("Starting controller, listening on address: %s", address)

	lis, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	s := grpc.NewServer()

	pb.RegisterControllerServer(s, &server{})

	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
