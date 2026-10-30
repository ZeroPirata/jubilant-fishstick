package discovery

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// Em 429 (limite de pedidos), espera e tenta de novo em vez de perder o resto da lista.
var (
	retryAttempts = 4
	retryBase     = 20 * time.Second // recuo sem Retry-After: 20s, 40s, 60s
	retryMax      = 2 * time.Minute
	retryFloor    = time.Second
)

var errRateLimited = fmt.Errorf("limite de pedidos atingido")

// getWithRetry faz o GET. Em 429 respeita o Retry-After (ou recua cada vez mais) e
// avisa por onWait quanto vai esperar. Devolve errRateLimited se o limite persistir.
func getWithRetry(ctx context.Context, client *http.Client, url string, header http.Header, onWait func(time.Duration)) (*http.Response, error) {
	for attempt := 1; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header = header.Clone()
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusTooManyRequests {
			return resp, nil
		}
		delay := retryAfter(resp.Header.Get("Retry-After"), time.Duration(attempt)*retryBase)
		resp.Body.Close()
		if attempt >= retryAttempts {
			return nil, errRateLimited
		}
		if onWait != nil {
			onWait(delay)
		}
		if !wait(ctx, delay) {
			return nil, ctx.Err()
		}
	}
}

// retryAfter lê o cabeçalho (segundos ou data HTTP), limitado a retryMax.
func retryAfter(h string, fallback time.Duration) time.Duration {
	d := fallback
	if secs, err := strconv.Atoi(h); err == nil && secs >= 0 {
		d = time.Duration(secs) * time.Second
	} else if t, err := http.ParseTime(h); err == nil {
		d = time.Until(t)
	}
	return min(max(d, retryFloor), retryMax)
}
