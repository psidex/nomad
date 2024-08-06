package main

import (
	"context"
	"io"
	"log"
	"time"

	"github.com/psidex/nomad/internal/agent"
	pb "github.com/psidex/nomad/internal/controller/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	nomadVersion                 = 0
	controllerAddress            = "nomad-controller:50051"
	defaultSingleScrapeTimeoutMs = 10_000
	gRPCCallTimeout              = time.Second * 10
)

func main() {
	// TODO: An outer loop that reconnects to controller when disconnected, add a
	// "shutdown" command in the stream to allow graceful shutdown

	log.Printf("Connecting to controller at address: %s", controllerAddress)
	conn, err := grpc.NewClient(controllerAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Could not connect to controller: %s", err)
	}
	defer conn.Close()

	controller := pb.NewControllerClient(conn)

	worker := agent.Worker{}

	log.Println("Initiating worker stream with controller")
	stream, err := controller.WorkerStream(context.Background())
	if err != nil {
		log.Fatalf("Create worker stream error: %s", err)
	}

	log.Println("Handshaking with controller")
	err = stream.Send(
		&pb.WorkerMessage{
			Message: &pb.WorkerMessage_Handshake{
				Handshake: &pb.WorkerHandshake{
					NomadVersion: nomadVersion,
				},
			},
		},
	)
	if err != nil {
		log.Fatalf("Failed to handshake with controller: %s", err)
	}

	for {
		var resp *pb.ControllerMessage
		resp, err = stream.Recv()
		if err == io.EOF || err != nil {
			log.Printf("Received err from worker stream: %s", err)
			break
		}

		switch msg := resp.Message.(type) {
		case *pb.ControllerMessage_ScrapeInstruction:
			for _, url := range msg.ScrapeInstruction.Urls {
				log.Printf("Scraping URL: %s", url)

				scrapedData := worker.ScrapeSinglePage(url)
				resp := &pb.WorkerMessage{
					Message: &pb.WorkerMessage_Data{
						Data: scrapedData,
					},
				}

				if err := stream.Send(resp); err != nil {
					log.Fatalf("Failed to send on worker stream: %s", err)
				}
			}

		case *pb.ControllerMessage_ConfigUpdate:
			log.Printf("Received worker config update: %+v", msg.ConfigUpdate)
			worker.Id = msg.ConfigUpdate.WorkerId
			worker.Cfg = msg.ConfigUpdate

		default:
			log.Fatalf("Received unknown message type from controller")
		}
	}

	if err := stream.CloseSend(); err != nil {
		log.Printf("Failed to close worker stream: %s", err)
	}

	log.Println("Done, goodbye")
}
