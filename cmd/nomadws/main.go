package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"github.com/psidex/nomad/internal/graphs/graphologyws"
	"github.com/psidex/nomad/internal/lib"
	"github.com/psidex/nomad/internal/nomad"
	"github.com/psidex/nomad/internal/webserver"
)

var (
	upgrader = websocket.Upgrader{}
)

func main() {
	upgrader.CheckOrigin = func(r *http.Request) bool { return true }

	staticDir := flag.String("d", "public", "the directory to serve static files from")
	address := flag.String("b", "127.0.0.1:8080", "the ip:port to bind the webserver to")

	flag.Parse()

	http.Handle("/", http.FileServer(http.Dir(*staticDir)))
	http.HandleFunc("/ws", nomadSession)

	log.Fatal(http.ListenAndServe(*address, nil))
}

func nomadSession(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("ws upgrade err:", err)
		return
	}
	defer c.Close()

	ws := lib.NewThreadSafeWebSocket(c)

	_, msg, err := ws.ReadMessage()
	if err != nil {
		log.Println("ws cfg read err:", err)
		return
	}

	cfg := &webserver.SessionConfig{}
	if err = json.Unmarshal(msg, cfg); err != nil {
		log.Println("ws cfg unmarshal err:", err)
		return
	}

	n := nomad.NewNomad(
		nomad.Config{
			WorkerCooldown: cfg.WorkerCooldown,
			WorkerCount:    cfg.WorkerCount,
			InitialUrls:    cfg.InitialUrls,
			RandomCrawl:    cfg.RandomCrawl,
		},
		&http.Client{
			Timeout: cfg.HttpClientTimeout.Duration,
		},
		graphologyws.NewGraphologyWs(ws),
	)

	if err := n.Run(); err != nil {
		log.Println("nomad run err:", err)
		return
	}

	wsrecv := make(chan struct{})
	timer := time.NewTimer(cfg.Runtime.Duration)

	go func() {
		// Client can send anything and it will cancel the session.
		// Warning: As this is the thread-safe version, this will block any other reads.
		_, _, _ = ws.ReadMessage()
		// If we never read a message, the outer function call will return, closing the
		// WS and causing ReadMessage to return an error, which will end this goroutine.
		close(wsrecv)
	}()

	select {
	case <-timer.C:
	case <-wsrecv:
	}

	n.Cancel()
}
