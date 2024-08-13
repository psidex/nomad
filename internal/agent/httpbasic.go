package agent

import (
	"io"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/net/html"

	"github.com/corpix/uarand"

	"github.com/psidex/nomad/internal/controller/pb"
)

type byteCounter struct {
	count *int64
}

func (b *byteCounter) Write(p []byte) (int, error) {
	n := len(p)
	*b.count += int64(n)
	return n, nil
}

func (w Worker) makeBasicHttpRequest(urlStr string) pb.ScrapedData {
	startTime := time.Now()

	baseURL, err := url.Parse(urlStr)
	if err != nil {
		w.logger.Error("Failed to parse request URL", "url", urlStr, "error", err)
		return pb.ScrapedData{Error: pb.ScrapeError_INVALID_REQUEST}
	}

	client := &http.Client{
		Timeout: time.Millisecond * time.Duration(w.cfg.SingleScrapeTimeoutMs),
	}

	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		w.logger.Error("Failed create HTTP request", "url", urlStr, "error", err)
		return pb.ScrapedData{Error: pb.ScrapeError_INVALID_REQUEST}
	}
	req.Header.Set("User-Agent", uarand.GetRandom())

	resp, err := client.Do(req)
	if err != nil {
		w.logger.Error("Failed to do HTTP request", "url", urlStr, "error", err)
		return pb.ScrapedData{Error: pb.ScrapeError_INVALID_REQUEST}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		w.logger.Error("HTTP request returned non-200 status code", "url", urlStr, "status", resp.StatusCode)
		return pb.ScrapedData{Error: pb.ScrapeError_INVALID_REQUEST}
	}

	var downloadedBytes int64
	tee := io.TeeReader(resp.Body, &byteCounter{&downloadedBytes})

	doc, err := html.Parse(tee)
	if err != nil {
		w.logger.Error("Failed to parse received HTML", "url", urlStr, "error", err)
		return pb.ScrapedData{Error: pb.ScrapeError_INVALID_REQUEST}
	}

	urls := extractURLs(doc, baseURL)

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
