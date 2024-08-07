package main

import (
	"context"
	"io"
	"log"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/psidex/nomad/internal/agent"
	pb "github.com/psidex/nomad/internal/controller/pb"
)

const (
	// Should always match the controller version
	nomadVersion int64 = 0

	// How long to wait between trying to reconnect to the controller
	reconnectSleep = time.Second * 3

	// How many times the worker stream send/recv can error before abandoning
	streamErrCountThreshold = 5

	// Default controller address, set using NOMAD_CONTROLLER_ADDRESS
	defaultControllerAddress = "nomad-controller:50051"
	// Default worker count, set using NOMAD_AGENT_WORKER_COUNT
	defaultWorkerCount = 1
)

// worker returns true/false to indicate if it should be called again (for reconnecting)
func worker(ctx context.Context, addr string) bool {
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
		log.Printf("Stopping: Failed to handshake with controller: %s", err)
		// If the handshake failed, the fix probably wont be a simple reconnect
		return false
	}

	streamErrCount := 0

mainLoop:
	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping: Context cancelled")
			return false
		default:
		}

		if streamErrCount >= streamErrCountThreshold {
			log.Printf("Stream err count %d >= %d, abandoning connection", streamErrCount, streamErrCountThreshold)
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
			log.Println("Stopping: Received shutdown from controller")
			return false

		default:
			log.Printf("Stopping: Received unknown message type from controller: %+v", resp)
			// We probably shouldn't try to reconnect if the controller is doing this
			return false
		}
	}
}

func workerReconnectLoop(ctx context.Context, wg *sync.WaitGroup, controllerAddress string) {
	defer wg.Done()
	log.Println("Worker starting")
	for {
		if !worker(ctx, controllerAddress) {
			break
		}
		log.Printf("Worker reconnecting in %s...", reconnectSleep)
		select {
		case <-ctx.Done():
			log.Println("Worker stopping due to context cancellation")
			return
		case <-time.After(reconnectSleep):
		}
	}
	log.Println("Worker stopped")
}

func main() {
	log.Println("Agent starting")

	controllerAddress := defaultControllerAddress
	if addr := os.Getenv("NOMAD_CONTROLLER_ADDRESS"); addr != "" {
		controllerAddress = addr
	}

	workerCount := defaultWorkerCount
	if count := os.Getenv("NOMAD_AGENT_WORKER_COUNT"); count != "" {
		var err error
		workerCount, err = strconv.Atoi(count)
		if err != nil || workerCount <= 0 {
			log.Fatalf("Invalid value for NOMAD_AGENT_WORKER_COUNT %v: %s", count, err)
		}
	}

	log.Printf("Configured worker count: %d", workerCount)

	ctx, cancel := context.WithCancel(context.Background())

	wg := &sync.WaitGroup{}
	wg.Add(workerCount)

	for i := 0; i < workerCount; i++ {
		go workerReconnectLoop(ctx, wg, controllerAddress)
	}

	// Listen for interrupt signal to gracefully shut down
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	log.Println("Agent shutting down")
	cancel()

	wg.Wait()
	log.Println("Agent stopped")
}
