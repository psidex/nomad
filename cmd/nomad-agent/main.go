package main

import (
	"context"
	"io"
	"log"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/psidex/nomad/internal/controller/pb"
)

const (
	nomadVersion = 0
	address      = "nomad-controller:50051"
)

func doWork() {
	// create context
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	totalSize := 0

	// Define a function to handle network response events
	handleResponseReceived := func(ctx context.Context) error {
		chromedp.ListenTarget(ctx, func(ev interface{}) {
			switch ev := ev.(type) {
			case *network.EventLoadingFinished:
				log.Println(ev.EncodedDataLength)
				totalSize += int(ev.EncodedDataLength)
			}
		})
		return nil
	}

	// run task list
	var res string
	err := chromedp.Run(ctx,
		network.Enable(),
		chromedp.ActionFunc(handleResponseReceived),
		chromedp.Navigate(`https://pkg.go.dev/time`),
		chromedp.Text(`.Documentation-overview`, &res, chromedp.NodeVisible),
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Println(strings.TrimSpace(res))
	log.Println(totalSize)
}

func main() {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	c := pb.NewControllerClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	agentResp, err := c.RegisterAgent(ctx, &pb.RegisterAgentRequest{NomadVersion: nomadVersion})
	if err != nil {
		log.Fatalf("Could not register agent: %v", err)
	}
	log.Printf("Agent ID: %v, HTTP Timeout: %v ms, Version Mismatch: %v",
		agentResp.AgentId, agentResp.Config, agentResp.VersionMismatch)

	if agentResp.VersionMismatch {
		log.Fatal("version mismatch")
	}

	log.Println("Asking for Scrape()")

	stream, err := c.Scrape(context.Background())
	if err != nil {
		log.Fatalf("open stream error %v", err)
	}

	log.Println("Asking for stream.Recv()")

	resp, err := stream.Recv()
	if err == io.EOF {
		log.Println("EOF, return")
		return
	}
	if err != nil {
		log.Fatalf("can not receive %v", err)
	}

	switch msg := resp.Message.(type) {
	case *pb.ScrapeStreamMessage_ScrapeInstruction:
		log.Printf("Got urls to scrape: %+v", msg.ScrapeInstruction.Urls)

		// for _, url := range msg.ScrapeInstruction.Urls {
		doWork()

		metrics := &pb.ScrapeMetrics{
			ResponseSizeBytes: 10000,
			HttpStatusCode:    200,
			NumFoundUrls:      5,
			ScrapeDurationMs:  150,
		}

		scrapeRequest := &pb.ScrapeStreamMessage{
			Message: &pb.ScrapeStreamMessage_ScrapeInformation{
				ScrapeInformation: &pb.ScrapeInformation{
					AgentId:    agentResp.AgentId,
					ScrapedUrl: "scrapedurl",
					FoundUrls:  []string{"https://example.com/page1", "https://example.com/page2"}, // Example values
					Metrics:    metrics,
					Error:      pb.URLRequestErrorCode_NONE,
				},
			},
		}

		log.Println("Sending scrapeRequest as response")

		if err := stream.Send(scrapeRequest); err != nil {
			log.Fatalf("can not send %v", err)
		}
		// }

	case *pb.ScrapeStreamMessage_ConfigUpdate:
		log.Printf("Received new configuration: %+v", msg.ConfigUpdate)

	default:
		log.Printf("Received unknown message type")
	}

	log.Println("Doing closesend")

	if err := stream.CloseSend(); err != nil {
		log.Println(err)
	}

	log.Println("Reported URL result successfully")
}
