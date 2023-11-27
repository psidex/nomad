package graphs

// GraphProvider defines an interface that can be used to track connections between
// hostnames.
type GraphProvider interface {
	// AddHostnameConnection should be thread-safe.
	AddHostnameConnection(fromHost, toHost string)
}

// CliGraphProvider extends the GraphProvider interface to accommodate CLI
// functionality.
type CliGraphProvider interface {
	GraphProvider

	// RenderToFile is not assumed to be thread-safe.
	// filename should be the desired file name without an extension.
	RenderToFile(filename string) error
}

// WebsocketGraphProvider extends the GraphProvider interface to accommodate WebSocket
// functionality.
type WebsocketGraphProvider interface {
	GraphProvider

	// These instruct the provider to send a message over the WebSocket to tell the
	// client that the given hostname is being crawled / is finished being crawled. This
	// is needed because only the provider knows the ID for a given hostname, which is
	// required to update the frontend correctly.
	NotifyStartCrawl(workerId uint, hostname string)
	NotifyEndCrawl(workerId uint, hostname string, deadEnd bool)
}
