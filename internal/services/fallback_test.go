package services

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"hackton-treino/config"
)

// fakeSource devolve uma cadeia fixa por purpose, como se viesse do painel.
type fakeSource map[Purpose][]Provider

func (f fakeSource) Providers(_ context.Context, purpose Purpose) ([]Provider, error) {
	return f[purpose], nil
}

// fakeLLM sobe um servidor que responde sempre com o status e body dados e conta as chamadas.
func fakeLLM(t *testing.T, status int, body string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func newFallbackService(cfg *config.Config, source ProviderSource) *AiService {
	return &AiService{
		Config:           cfg,
		Source:           source,
		httpClient:       &http.Client{},
		scrapeHttpClient: &http.Client{},
	}
}

var okCurriculo = string(validResponse(`{"resumo":"ok","experiencias":[],"cover_letter":"ok"}`))

func TestFallback_PrimeiroFalhaSegundoResponde(t *testing.T) {
	bad, badCalls := fakeLLM(t, http.StatusInternalServerError, "erro")
	good, goodCalls := fakeLLM(t, http.StatusOK, okCurriculo)

	svc := newFallbackService(&config.Config{}, fakeSource{PurposeMain: {
		{Name: "openai", URL: bad.URL, Model: "ia1"},
		{Name: "openai", URL: good.URL, Model: "ia2"},
	}})

	res, err := svc.GenerateCurriculum(context.Background(), "sys", "user")
	if err != nil {
		t.Fatalf("esperava sucesso via fallback, recebeu: %v", err)
	}
	if res.Resumo != "ok" {
		t.Errorf("Resumo = %q, want ok", res.Resumo)
	}
	if badCalls.Load() != 1 || goodCalls.Load() != 1 {
		t.Errorf("chamadas = (%d, %d), want (1, 1)", badCalls.Load(), goodCalls.Load())
	}
}

func TestFallback_PrimeiroOkNaoChamaOsOutros(t *testing.T) {
	good, _ := fakeLLM(t, http.StatusOK, okCurriculo)
	other, otherCalls := fakeLLM(t, http.StatusOK, okCurriculo)

	svc := newFallbackService(&config.Config{}, fakeSource{PurposeMain: {
		{Name: "openai", URL: good.URL},
		{Name: "openai", URL: other.URL},
	}})

	if _, err := svc.GenerateCurriculum(context.Background(), "sys", "user"); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if otherCalls.Load() != 0 {
		t.Errorf("segundo provider chamado %d vezes, want 0", otherCalls.Load())
	}
}

// Um 429 em quem não é o último passa a vez na hora, sem esperar nem repetir.
func TestFallback_RateLimitPassaAVezSemEsperar(t *testing.T) {
	limited, limitedCalls := fakeLLM(t, http.StatusTooManyRequests, `{}`) // sem retryDelay = espera de 30s se repetisse
	good, _ := fakeLLM(t, http.StatusOK, okCurriculo)

	svc := newFallbackService(&config.Config{}, fakeSource{PurposeMain: {
		{Name: "openai", URL: limited.URL},
		{Name: "openai", URL: good.URL},
	}})

	if _, err := svc.GenerateCurriculum(context.Background(), "sys", "user"); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if n := limitedCalls.Load(); n != 1 {
		t.Errorf("provider com 429 chamado %d vezes, want 1", n)
	}
}

// Só quando todos estouram o rate limit o worker deve pausar (ErrRateLimit).
// O último da cadeia mantém o retry original.
func TestFallback_TodosRateLimit(t *testing.T) {
	first, firstCalls := fakeLLM(t, http.StatusTooManyRequests, rateLimitBody)
	last, lastCalls := fakeLLM(t, http.StatusTooManyRequests, rateLimitBody)

	svc := newFallbackService(&config.Config{}, fakeSource{PurposeMain: {
		{Name: "openai", URL: first.URL, Model: "ia1"},
		{Name: "openai", URL: last.URL, Model: "ia2"},
	}})

	_, err := svc.GenerateCurriculum(context.Background(), "sys", "user")
	var errRL *ErrRateLimit
	if !errors.As(err, &errRL) {
		t.Fatalf("esperava *ErrRateLimit, recebeu %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "ia1") || !strings.Contains(err.Error(), "ia2") {
		t.Errorf("mensagem deveria citar os dois providers: %v", err)
	}
	if firstCalls.Load() != 1 || lastCalls.Load() != 3 {
		t.Errorf("chamadas = (%d, %d), want (1, 3)", firstCalls.Load(), lastCalls.Load())
	}
}

// Se um falhou por outro motivo, não é rate limit geral: o worker não deve pausar.
func TestFallback_RateLimitMaisErroComumNaoPausa(t *testing.T) {
	limited, _ := fakeLLM(t, http.StatusTooManyRequests, rateLimitBody)
	broken, _ := fakeLLM(t, http.StatusInternalServerError, "erro")

	svc := newFallbackService(&config.Config{}, fakeSource{PurposeMain: {
		{Name: "openai", URL: limited.URL},
		{Name: "openai", URL: broken.URL},
	}})

	_, err := svc.GenerateCurriculum(context.Background(), "sys", "user")
	if err == nil {
		t.Fatal("esperava erro")
	}
	var errRL *ErrRateLimit
	if errors.As(err, &errRL) {
		t.Errorf("não deveria ser ErrRateLimit: %v", err)
	}
}

// Resposta 200 com JSON inválido também conta como falha do modelo.
func TestFallback_JSONInvalidoTentaOProximo(t *testing.T) {
	garbage, _ := fakeLLM(t, http.StatusOK, string(validResponse("isso não é json")))
	good, goodCalls := fakeLLM(t, http.StatusOK, okCurriculo)

	svc := newFallbackService(&config.Config{}, fakeSource{PurposeMain: {
		{Name: "openai", URL: garbage.URL},
		{Name: "openai", URL: good.URL},
	}})

	if _, err := svc.GenerateCurriculum(context.Background(), "sys", "user"); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if goodCalls.Load() != 1 {
		t.Errorf("segundo provider chamado %d vezes, want 1", goodCalls.Load())
	}
}

func TestResolveProviders(t *testing.T) {
	cfg := &config.Config{
		Ai:       config.AiConfig{Provider: "gemini", Url: "http://main", Model: "env-main"},
		ScrapeAi: config.ScrapeAiConfig{Provider: "ollama", Url: "http://scrape", Model: "env-scrape"},
	}
	painel := []Provider{{Name: "openai", URL: "http://painel", Model: "painel"}}

	cases := []struct {
		label    string
		source   ProviderSource
		activate bool
		purpose  Purpose
		want     string
	}{
		{"main sem painel usa .env", nil, false, PurposeMain, "env-main"},
		{"main com painel vazio usa .env", fakeSource{}, false, PurposeMain, "env-main"},
		{"main com painel", fakeSource{PurposeMain: painel}, false, PurposeMain, "painel"},
		{"scrape desativado usa a cadeia principal", nil, false, PurposeScrape, "env-main"},
		{"scrape desativado herda o painel principal", fakeSource{PurposeMain: painel}, false, PurposeScrape, "painel"},
		{"scrape ativado usa .env de scrape", nil, true, PurposeScrape, "env-scrape"},
		{"scrape com painel ignora .env", fakeSource{PurposeScrape: painel}, true, PurposeScrape, "painel"},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			c := *cfg
			c.ScrapeAi.Activate = tc.activate
			svc := newFallbackService(&c, tc.source)
			got := svc.resolveProviders(context.Background(), tc.purpose)
			if len(got) != 1 || got[0].Model != tc.want {
				t.Errorf("resolveProviders = %+v, want modelo %q", got, tc.want)
			}
		})
	}
}

// A key do Gemini vai no header: na URL ela vazaria nos logs junto com erros de rede.
func TestBuildRequest_GeminiKeyNoHeader(t *testing.T) {
	svc := newFallbackService(&config.Config{}, nil)
	req, err := svc.buildRequest(context.Background(), []byte(`{}`), Provider{Name: "gemini", URL: "http://x/model:generateContent", Key: "segredo"})
	if err != nil {
		t.Fatalf("buildRequest: %v", err)
	}
	if strings.Contains(req.URL.String(), "segredo") {
		t.Errorf("key apareceu na URL: %s", req.URL)
	}
	if got := req.Header.Get("x-goog-api-key"); got != "segredo" {
		t.Errorf("x-goog-api-key = %q, want segredo", got)
	}
}

// llmReply monta uma resposta OpenAI-compatible com content e finish_reason específicos.
func llmReply(content, finish string) string {
	b, _ := json.Marshal(LLMResponse{Choices: []Choice{{
		Message:      Message{Role: "assistant", Content: content},
		FinishReason: finish,
	}}})
	return string(b)
}

var okScrape = llmReply(`{"Description":"Backend Go","Stack":["Go"],"Requirements":[]}`, "stop")

// Cenário de produção ("unexpected end of JSON input"): modelo de raciocínio gasta o
// limite pensando e devolve content vazio. Agora vira erro claro e cai no próximo.
func TestScrape_RespostaVaziaOuCortadaUsaFallback(t *testing.T) {
	cases := []struct {
		label, body, wantErr string
	}{
		{"content vazio", llmReply("", "stop"), "resposta vazia"},
		{"cortado por max_tokens", llmReply(`{"Description":"Backend`, "length"), "cortada pelo limite"},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			bad, _ := fakeLLM(t, http.StatusOK, tc.body)
			good, goodCalls := fakeLLM(t, http.StatusOK, okScrape)

			svc := newFallbackService(&config.Config{}, fakeSource{PurposeScrape: {
				{Name: "openai", URL: bad.URL, Model: "reasoning"},
				{Name: "openai", URL: good.URL, Model: "normal"},
			}})
			res, err := svc.GenerateScrapeSite(context.Background(), "Vaga para dev Go")
			if err != nil {
				t.Fatalf("esperava sucesso via fallback, recebeu: %v", err)
			}
			if goodCalls.Load() != 1 || len(res.Stack) != 1 {
				t.Errorf("fallback não usado corretamente: calls=%d res=%+v", goodCalls.Load(), res)
			}

			// Sozinho, o erro precisa dizer o motivo real.
			solo := newFallbackService(&config.Config{}, fakeSource{PurposeScrape: {{Name: "openai", URL: bad.URL, Model: "reasoning"}}})
			_, err = solo.GenerateScrapeSite(context.Background(), "Vaga para dev Go")
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("erro = %v, want conter %q", err, tc.wantErr)
			}
		})
	}
}

func TestScrape_IgnoraBlocoThink(t *testing.T) {
	srv, _ := fakeLLM(t, http.StatusOK, llmReply("<think>\nvou listar a stack {\n</think>\n"+`{"Description":"x","Stack":["Go","Kafka"],"Requirements":[]}`, "stop"))
	svc := newFallbackService(&config.Config{}, fakeSource{PurposeScrape: {{Name: "openai", URL: srv.URL, Model: "r1"}}})

	res, err := svc.GenerateScrapeSite(context.Background(), "Vaga")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(res.Stack) != 2 {
		t.Errorf("Stack = %v, want [Go Kafka]", res.Stack)
	}
}

// Erro de parse precisa trazer o começo da resposta, senão não dá para diagnosticar em produção.
func TestScrape_ErroDeParseMostraTrecho(t *testing.T) {
	srv, _ := fakeLLM(t, http.StatusOK, llmReply("Desculpe, não consigo ajudar com isso.", "stop"))
	svc := newFallbackService(&config.Config{}, fakeSource{PurposeScrape: {{Name: "openai", URL: srv.URL}}})

	_, err := svc.GenerateScrapeSite(context.Background(), "Vaga")
	if err == nil || !strings.Contains(err.Error(), "Desculpe") {
		t.Errorf("erro deveria conter o trecho da resposta: %v", err)
	}
}

// O bug original: o body do scrape dependia de description != "". Agora cada chamada
// monta o próprio body, com o prompt de extração e o limite de tokens folgado.
func TestBuildScrapeBody(t *testing.T) {
	svc := newFallbackService(&config.Config{}, nil)
	body := svc.buildScrapeBody("Vaga para dev Go", Provider{Name: "openai", Model: "m"})

	raw, _ := json.Marshal(body)
	s := string(raw)
	if !strings.Contains(s, "Vaga para dev Go") || !strings.Contains(s, "extrator de dados") {
		t.Errorf("body de scrape sem descrição ou sem prompt de extração: %s", s[:200])
	}
	if body["max_tokens"] != scrapeMaxTokens {
		t.Errorf("max_tokens = %v, want %d", body["max_tokens"], scrapeMaxTokens)
	}
}

// JSON válido no schema antigo ({"curriculo": ...}) não tem o que o worker precisa:
// precisa virar erro (e fallback), não um currículo vazio.
func TestCurriculo_SchemaAntigoViraErro(t *testing.T) {
	srv, _ := fakeLLM(t, http.StatusOK, string(validResponse(`{"curriculo":"texto","cover_letter":""}`)))
	svc := newFallbackService(&config.Config{}, fakeSource{PurposeMain: {{Name: "openai", URL: srv.URL}}})

	_, err := svc.GenerateCurriculum(context.Background(), "sys", "user")
	if err == nil || !strings.Contains(err.Error(), "sem conteúdo") {
		t.Errorf("erro = %v, want 'sem conteúdo'", err)
	}
}
