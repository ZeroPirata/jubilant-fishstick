// Testes em package services (whitebox) — acesso direto a campos privados do struct.
package services

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"hackton-treino/config"
)

// --- helpers de teste ---

// newTestService cria um AiService mínimo apontando para uma URL específica.
// O provider "claude" é o mais simples: sem header de API key obrigatório.
func newTestService(t *testing.T, url string, client *http.Client) *AiService {
	t.Helper()
	return &AiService{
		Config: &config.Config{
			Ai: config.AiConfig{
				Provider: "claude",
				Model:    "test-model",
				Url:      url,
			},
		},
		httpClient:       client,
		scrapeHttpClient: client,
	}
}

// spyBody envolve um io.ReadCloser e conta quantas vezes Close() foi chamado.
// Útil para verificar que o corpo da resposta HTTP foi fechado inline, não via defer.
type spyBody struct {
	io.ReadCloser
	closed atomic.Int32
}

func (s *spyBody) Close() error {
	s.closed.Add(1)
	return s.ReadCloser.Close()
}

// spyTransport substitui o body de cada resposta por um spyBody rastreável.
type spyTransport struct {
	base   http.RoundTripper
	bodies []*spyBody
}

func (t *spyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil || resp == nil {
		return resp, err
	}
	spy := &spyBody{ReadCloser: resp.Body}
	t.bodies = append(t.bodies, spy)
	resp.Body = spy
	return resp, nil
}

// validResponse retorna o JSON que o AiService espera receber da LLM.
func validResponse(content string) []byte {
	b, _ := json.Marshal(LLMResponse{
		Choices: []Choice{
			{Message: Message{Role: "assistant", Content: content}},
		},
	})
	return b
}

// rateLimitBody retorna um body 429 com retryDelay de -2s.
// parseRetryDelay vai calcular: -2s + 2s (margem) = 0 → time.After(0) dispara imediatamente.
// Isso evita sleeps reais durante os testes.
const rateLimitBody = `{"error":{"details":[{"@type":"type.googleapis.com/google.rpc.RetryInfo","retryDelay":"-2s"}]}}`

// --- testes ---

// TestSendRequest_BodyFechadoNoSucesso garante que o body é fechado inline (não via defer)
// mesmo na resposta de sucesso. O bug original usava defer dentro de um loop de retry,
// o que acumula closes e confunde o leitor — a correção fecha inline logo após o Decode.
func TestSendRequest_BodyFechadoNoSucesso(t *testing.T) {
	content := `{"curriculo":"ok","cover_letter":"ok"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(validResponse(content))
	}))
	defer srv.Close()

	spy := &spyTransport{base: http.DefaultTransport}
	svc := newTestService(t, srv.URL, &http.Client{Transport: spy})

	_, err := svc.sendRequest(context.Background(), []byte(`{}`), false)
	if err != nil {
		t.Fatalf("sendRequest: erro inesperado: %v", err)
	}

	if len(spy.bodies) == 0 {
		t.Fatal("nenhum body rastreado — o spy não funcionou")
	}
	for i, b := range spy.bodies {
		if b.closed.Load() == 0 {
			t.Errorf("body[%d] não foi fechado após resposta 200", i)
		}
	}
}

// TestSendRequest_BodyFechadoNoErro garante que o body também é fechado quando
// o servidor retorna status != 200 (ex: 500 Internal Server Error).
func TestSendRequest_BodyFechadoNoErro(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`erro interno`))
	}))
	defer srv.Close()

	spy := &spyTransport{base: http.DefaultTransport}
	svc := newTestService(t, srv.URL, &http.Client{Transport: spy})

	_, err := svc.sendRequest(context.Background(), []byte(`{}`), false)
	if err == nil {
		t.Fatal("esperava erro para status 500, mas não recebeu nenhum")
	}
	for i, b := range spy.bodies {
		if b.closed.Load() == 0 {
			t.Errorf("body[%d] não foi fechado após resposta de erro", i)
		}
	}
}

// TestSendRequest_RateLimitEsgota verifica que, após maxRetries tentativas com 429,
// a função retorna exatamente um *ErrRateLimit (não um erro genérico).
// O body de erro usa retryDelay: "-2s" para que a espera entre tentativas seja zero.
func TestSendRequest_RateLimitEsgota(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(rateLimitBody))
	}))
	defer srv.Close()

	svc := newTestService(t, srv.URL, &http.Client{Transport: http.DefaultTransport})

	_, err := svc.sendRequest(context.Background(), []byte(`{}`), false)

	// o erro deve ser do tipo *ErrRateLimit — não um erro genérico de string
	var errRL *ErrRateLimit
	if !errors.As(err, &errRL) {
		t.Fatalf("esperava *ErrRateLimit, recebeu %T: %v", err, err)
	}

	// deve ter tentado exatamente maxRetries (3) vezes
	const maxRetries = 3
	if n := calls.Load(); n != maxRetries {
		t.Errorf("número de tentativas = %d, want %d", n, maxRetries)
	}
}

// TestSendRequest_RateLimitDepoisRecupera verifica o cenário de retry bem-sucedido:
// os primeiros N calls retornam 429, o último retorna 200.
// O worker real reseta a flag do Redis e processa normalmente depois.
func TestSendRequest_RateLimitDepoisRecupera(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n < 3 {
			// primeiros 2 calls: 429
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(rateLimitBody))
			return
		}
		// terceiro call: sucesso
		w.Write(validResponse(`{"curriculo":"ok","cover_letter":"ok"}`))
	}))
	defer srv.Close()

	svc := newTestService(t, srv.URL, &http.Client{Transport: http.DefaultTransport})

	resp, err := svc.sendRequest(context.Background(), []byte(`{}`), false)
	if err != nil {
		t.Fatalf("sendRequest: esperava sucesso após retry, recebeu: %v", err)
	}
	if len(resp.Choices) == 0 {
		t.Fatal("resposta não tem choices")
	}
}

// TestSendRequest_ContextCanceladoDuranteRateLimit verifica que o context.Done()
// é respeitado durante a espera de retry — a função deve retornar ctx.Err(), não ErrRateLimit.
func TestSendRequest_ContextCanceladoDuranteRateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// retorna 429 com delay longo (fallback 30s) para forçar a espera
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{}`)) // body sem retryDelay → fallback 30s
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())

	svc := newTestService(t, srv.URL, &http.Client{Transport: http.DefaultTransport})

	// cancela o context logo depois do primeiro 429 ser recebido
	// (uma goroutine cancela enquanto a função espera o delay de 30s)
	go cancel()

	_, err := svc.sendRequest(ctx, []byte(`{}`), false)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("esperava context.Canceled, recebeu: %v", err)
	}
}

// TestSendRequest_GeminiParsing verifica que o formato de resposta Gemini
// é normalizado para o mesmo LLMResponse que os outros providers usam.
func TestSendRequest_GeminiParsing(t *testing.T) {
	geminiBody := `{
		"candidates": [{
			"content": {
				"parts": [{"text": "resposta do gemini"}]
			}
		}]
	}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(geminiBody))
	}))
	defer srv.Close()

	svc := &AiService{
		Config: &config.Config{
			Ai: config.AiConfig{
				Provider: "gemini",
				Model:    "gemini-test",
				Url:      srv.URL,
			},
		},
		httpClient:       &http.Client{Transport: http.DefaultTransport},
		scrapeHttpClient: &http.Client{Transport: http.DefaultTransport},
	}

	resp, err := svc.sendRequest(context.Background(), []byte(`{}`), false)
	if err != nil {
		t.Fatalf("sendRequest gemini: %v", err)
	}
	if got := resp.Choices[0].Message.Content; got != "resposta do gemini" {
		t.Errorf("content = %q, want %q", got, "resposta do gemini")
	}
}
