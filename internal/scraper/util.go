package scraper

import (
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

var userAgents = []string{
	// Chrome Windows
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/122.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/121.0.6167.85 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36",
	// Chrome Linux
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 Chrome/122.0.0.0 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 Chrome/120.0.6099.109 Safari/537.36",

	// Chrome Mac
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 13_5) AppleWebKit/537.36 Chrome/121.0.0.0 Safari/537.36",

	// Firefox
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:124.0) Gecko/20100101 Firefox/124.0",
	"Mozilla/5.0 (X11; Linux x86_64; rv:123.0) Gecko/20100101 Firefox/123.0",

	// Safari Mac
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 13_5) AppleWebKit/605.1.15 Version/17.0 Safari/605.1.15",

	// Android
	"Mozilla/5.0 (Linux; Android 13; SM-G991B) AppleWebKit/537.36 Chrome/122.0.0.0 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 12; Pixel 6) AppleWebKit/537.36 Chrome/121.0.0.0 Mobile Safari/537.36",

	// iPhone
	"Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 Version/17.0 Mobile/15E148 Safari/604.1",
}

func randomUserAgent() string {
	return userAgents[rand.Intn(len(userAgents))]
}

func applyHeaders(req *http.Request) {
	req.Header.Set("User-Agent", randomUserAgent())
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,pt-BR;q=0.8,pt;q=0.7")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
}

func doRequestWithRetry(client *http.Client, req *http.Request, maxRetries int) (*http.Response, error) {
	var resp *http.Response
	var err error

	for i := range maxRetries {
		applyHeaders(req)

		resp, err = client.Do(req)
		if err == nil && resp.StatusCode < 500 {
			return resp, nil
		}

		time.Sleep(time.Duration(500*(i+1)) * time.Millisecond)
	}

	return nil, fmt.Errorf("falha após %d tentativas: %v", maxRetries, err)
}

func cleanString(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
}
