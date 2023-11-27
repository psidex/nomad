package graphologyws

import "fmt"

func startCrawlNotification(workerId uint, hostnameId int) []byte {
	return []byte(fmt.Sprintf(
		`{"type": "startcrawl", "worker": "%d", "data": {"key": "%d"}}`,
		workerId, hostnameId,
	))
}

func endCrawlNotification(workerId uint, hostnameId int, deadEnd bool) []byte {
	return []byte(fmt.Sprintf(
		`{"type": "endcrawl", "worker": "%d", "data": {"key": "%d", "deadend": %t}}`,
		workerId, hostnameId, deadEnd,
	))
}
