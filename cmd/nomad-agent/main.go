package main

import (
	"context"
	"io"
	"log"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/psidex/nomad/internal/agent"
	pb "github.com/psidex/nomad/internal/controller/pb"
)

const (
	nomadVersion            int64 = 0
	reconnectSleep                = time.Second * 3
	streamErrCountThreshold       = 5
)

// run returns true/false to indicate if it should be called again (for reconnecting)
func run(addr string) bool {
	log.Printf("Connecting to controller at address: %s", addr)
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Printf("Could not connect to controller: %s", err)
		return true
	}
	defer func() { _ = conn.Close() }()

	controller := pb.NewControllerClient(conn)
	worker := agent.Worker{}

	log.Println("Initiating worker stream with controller")
	stream, err := controller.WorkerStream(context.Background())
	if err != nil {
		log.Printf("Error creating worker stream: %s", err)
		return true
	}
	defer func() { _ = stream.CloseSend() }()

	log.Println("Handshaking with controller")
	if err = stream.Send(
		&pb.WorkerMessage{
			Message: &pb.WorkerMessage_Handshake{
				Handshake: &pb.WorkerHandshake{
					NomadVersion: nomadVersion,
				},
			},
		},
	); err != nil {
		log.Printf("Failed to handshake with controller: %s", err)
		return true
	}

	streamErrCount := 0

mainLoop:
	for {
		if streamErrCount >= streamErrCountThreshold {
			return true
		}

		var resp *pb.ControllerMessage
		resp, err = stream.Recv()
		if err == io.EOF || err != nil {
			log.Printf("Received err from worker stream: %s", err)
			streamErrCount++
			continue
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
					log.Printf("Failed to send on worker stream: %s", err)
					streamErrCount++
					continue mainLoop
				}
			}

		case *pb.ControllerMessage_ConfigUpdate:
			log.Printf("Received worker config update: %+v", msg.ConfigUpdate)
			worker.Id = msg.ConfigUpdate.WorkerId
			worker.Cfg = msg.ConfigUpdate

		case *pb.ControllerMessage_Shutdown:
			log.Println("Received shutdown from controller:")
			return false

		default:
			log.Printf("Received unknown message type from controller")
			// Something has probably gone quite wrong, don't try to reconnect
			return false
		}
	}
}

func main() {
	controllerAddress := "nomad-controller:50051"
	if addr := os.Getenv("NOMAD_CONTROLLER_ADDRESS"); addr != "" {
		controllerAddress = addr
	}
	for {
		if run(controllerAddress) {
			time.Sleep(reconnectSleep)
			continue
		}
		break
	}
	log.Println("Stopped")
}
