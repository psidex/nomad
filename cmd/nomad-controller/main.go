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
}

func (s *server) Scrape(srv pb.Controller_ScrapeServer) error {
	ctx := srv.Context()
	log.Println("Scrape called")
	defer log.Println("Scrape returning out of func")

	for {
		log.Println("Scrape loop start")

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		resp := pb.ScrapeStreamMessage{
			Message: &pb.ScrapeStreamMessage_ScrapeInstruction{
				ScrapeInstruction: &pb.ScrapeInstruction{Urls: []string{"test"}},
			},
		}
		if err := srv.Send(&resp); err != nil {
			log.Printf("send error %v", err)
		}

		req, err := srv.Recv()
		if err == io.EOF {
			// return will close stream from server side
			log.Println("EOF exit")
			return nil
		}
		if err != nil {
			log.Printf("receive error %v", err)
			continue
		}

		log.Printf("scrape loop end, recvd: %+v", req)
	}
}

func (s *server) RegisterAgent(ctx context.Context, in *pb.RegisterAgentRequest) (*pb.RegisterAgentResponse, error) {
	if in.NomadVersion != nomadVersion {
		return &pb.RegisterAgentResponse{
			AgentId: 0,
			Config: &pb.AgentConfigUpdate{
				SingleScrapeTimeoutMs: 0,
			},
			VersionMismatch: true,
		}, nil
	}
	return &pb.RegisterAgentResponse{
		AgentId: 0,
		Config: &pb.AgentConfigUpdate{
			SingleScrapeTimeoutMs: 5000,
		},
		VersionMismatch: false,
	}, nil
}

func main() {
	lis, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	s := grpc.NewServer()

	pb.RegisterControllerServer(s, &server{})
	log.Printf("Server listening on %v", lis.Addr())

	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
