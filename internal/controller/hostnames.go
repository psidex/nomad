package controller

import (
	"net/url"
)

// getHostname takes a URL and returns just the hostname.
// For example: https://google.com -> google.com
func getHostname(urlStr string) (string, error) {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}
	return parsedURL.Hostname(), nil
}

// getHostnameAsUrl takes a URL and returns the hostname as a URL.
// For example: https://google.com/a/b/c?d=1#23 -> https://google.com
func getHostnameAsUrl(urlStr string) (string, error) {
	urlStrParsed, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}
	hostURL := &url.URL{
		Scheme: urlStrParsed.Scheme,
		Host:   urlStrParsed.Host,
	}
	return hostURL.String(), nil
}
