package agent

import (
	"context"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"

	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"

	"github.com/psidex/nomad/internal/controller/pb"
)

func (w Worker) makeChromeRequest(urlStr string) pb.ScrapedData {
	startTime := time.Now()

	baseURL, err := url.Parse(urlStr)
	if err != nil {
		w.logger.Error("Failed to parse request URL", "url", urlStr, "error", err)
		return pb.ScrapedData{Error: pb.ScrapeError_INVALID_REQUEST}
	}

	// Create a new context off of the master, opens a new tab(?)
	newTabCtx, newTabCancel := chromedp.NewContext(w.chromedpCtx)
	defer newTabCancel()

	// Add a timeout
	ctx, cancel := context.WithTimeout(
		newTabCtx,
		time.Millisecond*time.Duration(w.cfg.SingleScrapeTimeoutMs),
	)
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
	err = chromedp.Run(ctx,
		network.Enable(),
		chromedp.ActionFunc(countBytesAction),
		chromedp.Navigate(urlStr),
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
		w.logger.Error("chromedp.Run failed", "url", urlStr, "err", err)
		return pb.ScrapedData{Error: pb.ScrapeError_TIMEOUT}
	}

	parsed, err := html.Parse(strings.NewReader(pageSource))
	if err != nil {
		w.logger.Error("Failed to parse received HTML", "url", urlStr, "error", err)
		return pb.ScrapedData{Error: pb.ScrapeError_INVALID_REQUEST}
	}

	urls := extractURLs(parsed, baseURL)

	duration := int32(time.Since(startTime).Milliseconds())

	metrics := &pb.ScrapeMetrics{
		ResponseSizeBytes: downloadedBytes,
		NumFoundUrls:      int32(len(urls)),
		ScrapeDurationMs:  duration,
	}

	return pb.ScrapedData{
		AgentId:    w.id,
		ScrapedUrl: urlStr,
		FoundUrls:  urls,
		Metrics:    metrics,
		Error:      pb.ScrapeError_NONE,
	}
}
