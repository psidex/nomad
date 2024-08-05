package main

import (
	"context"
	"io"
	"log"
	"net"

	pb "github.com/psidex/nomad/internal/controller/pb"
	"google.golang.org/grpc"
)

const (
	nomadVersion = 0
	address      = "0.0.0.0:50051"
)

type server struct {
	pb.UnimplementedControllerServer
	agentCount int32
}

func (s *server) Scrape(srv pb.Controller_ScrapeServer) error {
	ctx := srv.Context()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		resp := pb.ControllerInstruction{
			Message: &pb.ControllerInstruction_ScrapeInstruction{
				ScrapeInstruction: &pb.ScrapeInstruction{Urls: []string{"https://pkg.go.dev/time"}},
			},
		}
		if err := srv.Send(&resp); err != nil {
			log.Printf("Failed to send on scrape stream: %s", err)
		}

		req, err := srv.Recv()
		if err == io.EOF {
			log.Println("Received EOF on scrape stream")
			break
		}
		if err != nil {
			log.Printf("Received error on scrape stream: %s", err)
			continue
		}

		log.Printf("Debug: scrape loop end, recvd: %+v", req)
		break
	}

	log.Printf("Debug: scrape function end")
	return nil
}

func (s *server) RegisterAgent(ctx context.Context, in *pb.RegisterAgentRequest) (*pb.RegisterAgentResponse, error) {
	if in.NomadVersion != nomadVersion {
		return &pb.RegisterAgentResponse{
			AgentId: 0,
			Config: &pb.AgentConfig{
				SingleScrapeTimeoutMs: 0,
			},
			VersionMismatch: true,
		}, nil
	}
	s.agentCount += 1
	return &pb.RegisterAgentResponse{
		AgentId: s.agentCount,
		Config: &pb.AgentConfig{
			SingleScrapeTimeoutMs: 10_000,
		},
		VersionMismatch: false,
	}, nil
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
