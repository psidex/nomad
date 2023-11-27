package nomad

import (
	"net/url"

	"golang.org/x/net/html"
)

// extractURLs gets all hrefs from all <a> tags in a html document as absolute URLs.
func extractURLs(n *html.Node, baseURL *url.URL) []string {
	var urls []string

	var visitNode func(*html.Node)
	visitNode = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					parsedURL, err := url.Parse(attr.Val)
					if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
						break
					}
					// If parsedURL is an absolute URL, parsedURL is returned
					// Else, resolve the relative URL to an absolute using baseURL
					absoluteURL := baseURL.ResolveReference(parsedURL).String()
					urls = append(urls, absoluteURL)
					break
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			visitNode(c)
		}
	}

	visitNode(n)

	return urls
}
