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

	log.Println("Registering agent with controller")
	ctx, cancel := context.WithTimeout(context.Background(), gRPCCallTimeout)
	defer cancel()
	agentResp, err := controller.RegisterAgent(ctx, &pb.RegisterAgentRequest{NomadVersion: nomadVersion})
	if err != nil {
		log.Fatalf("Could not register agent: %s", err)
	}
	if agentResp.VersionMismatch {
		log.Fatal("Received version mismatch from controller")
	}

	log.Printf("Received agent ID from controller: %d", agentResp.AgentId)
	log.Printf("Received agent config from controller: %+v", agentResp.Config)
	worker := agent.NewWorker(agentResp.AgentId, agentResp.Config)

	log.Println("Initiating scrape stream with controller")
	scrapeStream, err := controller.Scrape(context.Background())
	if err != nil {
		log.Fatalf("Create scrape stream error: %s", err)
	}

	log.Println("Starting work loop")
	for {
		var resp *pb.ControllerInstruction
		resp, err = scrapeStream.Recv()
		if err == io.EOF || err != nil {
			log.Printf("Received err from scrape stream: %s", err)
			break
		}

		switch msg := resp.Message.(type) {
		case *pb.ControllerInstruction_ScrapeInstruction:
			for _, url := range msg.ScrapeInstruction.Urls {
				log.Printf("Scraping URL: %s", url)
				scrapeRequest := worker.ScrapeSinglePage(url)
				if err := scrapeStream.Send(scrapeRequest); err != nil {
					log.Fatalf("Failed to send on scrape stream: %s", err)
				}
			}

		case *pb.ControllerInstruction_ConfigUpdate:
			log.Printf("Received agent config update from controller: %+v", msg.ConfigUpdate)
			worker.SetCfg(msg.ConfigUpdate)

		default:
			// TODO: Exit from loop here?
			log.Printf("Received unknown message type from controller")
		}
	}

	if err := scrapeStream.CloseSend(); err != nil {
		log.Printf("Failed to close scrape stream: %s", err)
	}

	log.Println("Done, goodbye")
}
