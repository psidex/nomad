package agent

import (
	"context"
	"log"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"

	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"

	"github.com/psidex/nomad/internal/controller/pb"
)

type Worker struct {
	id  int32
	cfg *pb.AgentConfig
}

func NewWorker(id int32, cfg *pb.AgentConfig) *Worker {
	return &Worker{id, cfg}
}

func (w *Worker) SetCfg(cfg *pb.AgentConfig) {
	w.cfg = cfg
}

func (w Worker) ScrapeSinglePage(urlToScrape string) *pb.ScrapeInformation {
	// Time how long our scrape operation takes
	startTime := time.Now()

	// TODO: is this a correct way to do the chromedp context?
	// Create context an ensure any long running Chrome tasks are cancelled when we exit
	timeoutCtx, timeoutCancel := context.WithTimeout(
		context.Background(),
		time.Millisecond*time.Duration(w.cfg.SingleScrapeTimeoutMs),
	)
	defer timeoutCancel()

	ctx, cancel := chromedp.NewContext(timeoutCtx)
	defer cancel()

	downloadedBytes := int64(0)

	countBytesAction := func(ctx context.Context) error {
		chromedp.ListenTarget(ctx, func(ev interface{}) {
			switch ev := ev.(type) {
			case *network.EventLoadingFinished:
				downloadedBytes += int64(ev.EncodedDataLength)
			}
		})
		return nil
	}

	var pageSource string
	err := chromedp.Run(ctx,
		network.Enable(),
		chromedp.ActionFunc(countBytesAction),
		chromedp.Navigate(urlToScrape),
		chromedp.ActionFunc(func(ctx context.Context) error {
			node, err := dom.GetDocument().Do(ctx)
			if err != nil {
				return err
			}
			pageSource, err = dom.GetOuterHTML().WithNodeID(node.NodeID).Do(ctx)
			return err
		}),
	)
	if err != nil {
		log.Fatal(err)
	}

	baseURL, err := url.Parse(urlToScrape)
	if err != nil {
		log.Fatal(err)
	}

	parsed, err := html.Parse(strings.NewReader(pageSource))
	if err != nil {
		log.Fatal(err)
	}

	urls := ExtractURLs(parsed, baseURL)

	duration := int32(time.Since(startTime).Milliseconds())

	metrics := &pb.ScrapeMetrics{
		ResponseSizeBytes: downloadedBytes,
		NumFoundUrls:      int32(len(urls)),
		ScrapeDurationMs:  duration,
	}

	return &pb.ScrapeInformation{
		AgentId:    w.id,
		ScrapedUrl: urlToScrape,
		FoundUrls:  urls,
		Metrics:    metrics,
		Error:      pb.URLRequestErrorCode_NONE,
	}
}
