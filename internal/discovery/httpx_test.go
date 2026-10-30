package discovery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestGetWithRetry(t *testing.T) {
	defer func(b, f time.Duration) { retryBase, retryFloor = b, f }(retryBase, retryFloor)
	retryBase, retryFloor = time.Millisecond, time.Millisecond

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) <= 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	var waits []time.Duration
	resp, err := getWithRetry(context.Background(), srv.Client(), srv.URL, http.Header{}, func(d time.Duration) { waits = append(waits, d) })
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if calls.Load() != 3 || len(waits) != 2 {
		t.Errorf("esperava 2 esperas e 3 pedidos: %d pedidos, esperas %v", calls.Load(), waits)
	}

	calls.Store(-100) // sempre 429
	if _, err := getWithRetry(context.Background(), srv.Client(), srv.URL, http.Header{}, nil); err != errRateLimited {
		t.Errorf("limite persistente deveria dar errRateLimited, deu %v", err)
	}
}

func TestRetryAfter(t *testing.T) {
	if d := retryAfter("7", time.Minute); d != 7*time.Second {
		t.Errorf("segundos: %v", d)
	}
	if d := retryAfter("", 30*time.Second); d != 30*time.Second {
		t.Errorf("sem cabeçalho usa o recuo: %v", d)
	}
	if d := retryAfter("9999", time.Second); d != retryMax {
		t.Errorf("limitado a retryMax: %v", d)
	}
}
