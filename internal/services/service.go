package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"hackton-treino/config"
	"hackton-treino/internal/resume"
	"hackton-treino/internal/scraper"
	"io"
	"net/http"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// NewLLMService cria o serviço de IA. source pode ser nil: nesse caso só o .env é usado.
func NewLLMService(cfg *config.Config, source ProviderSource) *AiService {
	timeout := config.GetConfigDurationOrDefault(cfg.Ai.Timeout, 3600*time.Second)
	scrapeTimeout := config.GetConfigDurationOrDefault(cfg.ScrapeAi.Timeout, 120*time.Second)
	return &AiService{
		Config: cfg,
		Source: source,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		scrapeHttpClient: &http.Client{
			Timeout: scrapeTimeout,
		},
	}
}

func (ai *AiService) buildRequest(ctx context.Context, payload []byte, p Provider) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.URL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("AiService: failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	switch {
	case p.Key == "" || p.Name == "ollama":
	case p.Name == "gemini":
		// Header em vez de ?key=: a URL aparece em mensagens de erro e iria parar nos logs.
		req.Header.Set("x-goog-api-key", p.Key)
	default:
		req.Header.Set("Authorization", "Bearer "+p.Key)
	}

	return req, nil
}

type AIInput struct {
	SystemPrompt string
	UserPrompt   string
}

// Limites de saída. Folgados de propósito: só são cobrados os tokens realmente gerados,
// e modelos de raciocínio gastam parte do limite "pensando" antes de responder —
// com 800 eles devolviam conteúdo vazio.
const (
	scrapeMaxTokens = 4096
	mainMaxTokens   = 8192
)

// buildScrapeBody monta o body da extração de dados da vaga (stack, requisitos, resumo).
func (ai *AiService) buildScrapeBody(description string, p Provider) map[string]any {
	scrapeProvider := p.Name
	scrapeModel := p.Model

	systemPrompt := `Você é um extrator de dados técnico. Retorne APENAS JSON válido, sem texto antes ou depois.

SCHEMA:
{
  "Description": "Resumo de 1-2 frases do papel",
  "Stack": ["um item por elemento"],
  "Requirements": ["requisito soft"]
}

REGRA CRÍTICA — ANTI-ALUCINAÇÃO:
Extraia APENAS tecnologias, ferramentas e domínios que aparecem LITERALMENTE no texto da vaga.
É ESTRITAMENTE PROIBIDO inferir, extrapolar ou adicionar qualquer item não descrito explicitamente.

REGRA FUNDAMENTAL — um item por elemento:
Nunca agrupe múltiplas tecnologias em um único elemento do array.
ERRADO: ["DevOps Tools (Docker, Kubernetes, GitLab CI/CD)"]
CERTO:  ["Docker", "Kubernetes", "GitLab CI/CD"]

REGRA DE SEPARAÇÃO — barras e listas inline:
Quando o texto usar "/" ou "," para listar itens juntos, separe cada um em elemento próprio.
ERRADO: ["RESTful/gRPC APIs"]  →  CERTO: ["gRPC", "REST"]
ERRADO: ["Kafka, RabbitMQ, or NATS"]  →  CERTO: ["Kafka", "RabbitMQ", "NATS"]

REGRA DE COMPLETUDE — não omita itens:
Se o texto menciona "X, Y e Z", todos os três entram. "etc." no final NÃO cancela os itens já listados.
ERRADO (omitir o último): texto diz "SOLID, clean code, clean architecture etc." → Stack tem só ["SOLID", "Clean Code"]
CERTO: ["SOLID", "Clean Code", "Clean Architecture"]

REGRA DE SUFIXO — remova sufixos genéricos:
Remova "API", "APIs", "Service", "Services" quando forem sufixo genérico, não parte do nome oficial.
"gRPC APIs" → "gRPC" · "RESTful APIs" → "REST" · "GraphQL" permanece (nome oficial)

REGRA DE RÓTULOS GENÉRICOS:
"Boas Práticas" ou "Best Practices" como rótulo isolado → omita.
Se vier seguido de itens nomeados, extraia cada item: "boas práticas como SOLID, clean code" → ["SOLID", "Clean Code"]

REGRA DE CONCEITOS ARQUITETURAIS — NÃO vão para Stack:
Termos como "microservices", "distributed systems", "high availability", "scalability", "APIs" isolado,
"databases" isolado, "cloud" isolado, "database optimization", "system design" são conceitos, não tecnologias.
Omita-os completamente — não vão nem para Stack nem para Requirements.
Se o texto menciona um sistema específico por nome (Kafka, PostgreSQL, AWS, Docker), extraia o nome. Do contrário, ignore.

Stack inclui SOMENTE tecnologias concretas e nomeadas: linguagens de programação, frameworks, bibliotecas, bancos de dados,
plataformas cloud (serviços específicos como "AWS S3", "GCP", "Azure"), ferramentas de CI/CD, observabilidade e contêineres,
protocolos com nome próprio (gRPC, GraphQL, REST, WebSocket), tecnologias de mensageria específicas (Kafka, RabbitMQ, NATS).

Requirements inclui APENAS: anos de experiência, idiomas, formação acadêmica.

IDIOMA: extraia os termos como aparecem no texto. Não traduza nem normalize — a normalização é feita pelo sistema depois.

EXEMPLO PT-BR:
Input: "Dev Backend. 3 anos com Golang e Postgres. Experiência com mensageria e microsserviços. AWS. Inglês fluente."
Output: {"Description":"Desenvolvedor Backend com foco em nuvem.","Stack":["Golang","PostgreSQL","AWS"],"Requirements":["3 anos de experiência","Inglês fluente"]}
Nota: "mensageria" e "microsserviços" são conceitos — omitidos. Nenhum sistema de mensageria específico foi citado.

EXEMPLO EN:
Input: "Experience with RESTful/gRPC APIs, PostgreSQL, Docker. Kafka, RabbitMQ, or NATS is a plus. Distributed systems knowledge required."
Output: {"Description":"Backend engineer with API and messaging experience.","Stack":["REST","gRPC","PostgreSQL","Docker","Kafka","RabbitMQ","NATS"],"Requirements":[]}
Nota: "Distributed systems knowledge" é conceito — omitido.`

	userMsg := "Analise o seguinte texto de vaga:\n\n" + description

	if scrapeProvider == "gemini" {
		return map[string]any{
			"system_instruction": map[string]any{
				"parts": []map[string]any{{"text": systemPrompt}},
			},
			"contents": []map[string]any{
				{"role": "user", "parts": []map[string]any{{"text": userMsg}}},
			},
			"generationConfig": map[string]any{
				"temperature":      0.0,
				"responseMimeType": "application/json",
				"maxOutputTokens":  scrapeMaxTokens,
			},
		}
	}

	if scrapeProvider == "ollama" {
		return map[string]any{
			"model":  scrapeModel,
			"stream": false,
			"format": "json",
			"options": map[string]any{
				"temperature": 0.0,
				"num_predict": scrapeMaxTokens,
				"seed":        42,
			},
			"messages": []map[string]any{
				{"role": "system", "content": systemPrompt},
				{"role": "user", "content": userMsg},
			},
		}
	}

	// OpenAI-compatible (nvidia, groq, openai, etc.)
	return map[string]any{
		"model":  scrapeModel,
		"stream": false,
		"response_format": map[string]string{"type": "json_object"},
		"temperature": 0.0,
		"max_tokens":  scrapeMaxTokens,
		"messages": []map[string]any{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userMsg},
		},
	}
}

func (ai *AiService) buildBody(input AIInput, p Provider) map[string]any {
	if p.Name == "gemini" {
		return map[string]any{
			"system_instruction": map[string]any{
				"parts": []map[string]any{{"text": input.SystemPrompt}},
			},
			"contents": []map[string]any{
				{"role": "user", "parts": []map[string]any{{"text": input.UserPrompt}}},
			},
			"generationConfig": map[string]any{
				"temperature":      0.7,
				"responseMimeType": "application/json",
				"maxOutputTokens":  mainMaxTokens,
			},
		}
	}

	body := map[string]any{
		"model":      p.Model,
		"max_tokens": mainMaxTokens,
		"messages": []map[string]any{
			{"role": "system", "content": input.SystemPrompt},
			{"role": "user", "content": input.UserPrompt},
		},
	}

	return ai.applyJsonFormat(body, p.Name)
}

func (ai *AiService) applyJsonFormat(body map[string]any, provider string) map[string]any {
	body["stream"] = false

	if provider == "ollama" {
		body["format"] = "json"
	} else {
		body["response_format"] = map[string]string{"type": "json_object"}
	}

	return body
}

// GenerateCurriculum percorre a cadeia de providers principal até um gerar um JSON válido.
func (ai *AiService) GenerateCurriculum(ctx context.Context, systemPrompt, userPrompt string) (*resume.LLMOutput, error) {
	var result *resume.LLMOutput
	err := ai.withFallback(ctx, PurposeMain, func(ctx context.Context, p Provider, last bool) (Usage, error) {
		r, usage, err := ai.generateCurriculum(ctx, p, last, systemPrompt, userPrompt)
		result = r
		return usage, err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (ai *AiService) generateCurriculum(ctx context.Context, p Provider, waitOn429 bool, systemPrompt, userPrompt string) (*resume.LLMOutput, Usage, error) {
	body := ai.buildBody(AIInput{SystemPrompt: systemPrompt, UserPrompt: userPrompt}, p)
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, Usage{}, fmt.Errorf("AiService: error in generate a json for body: %w", err)
	}

	raw, err := ai.sendRequest(ctx, ai.httpClient, payload, p, waitOn429)
	if err != nil {
		return nil, Usage{}, err
	}

	text, err := extractContent(raw)
	if err != nil {
		return nil, raw.Usage, err
	}
	content := sanitizeJSONLiterals(stripMarkdownCode(text))
	var result resume.LLMOutput
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, raw.Usage, fmt.Errorf("AiService: %w: %v (início da resposta: %q)", ErrParse, err, snippet(content))
	}
	if result.Error != "" {
		return nil, raw.Usage, fmt.Errorf("AiService: llm error: %s", result.Error)
	}
	// JSON válido mas sem conteúdo (ex.: schema antigo ou modelo que ignorou o formato).
	if result.Resumo == "" && result.CoverLetter == "" && len(result.Experiencias) == 0 {
		return nil, raw.Usage, fmt.Errorf("AiService: %w (início da resposta: %q)", ErrNoContent, snippet(content))
	}
	return &result, raw.Usage, nil
}

// GenerateScrapeSite percorre a cadeia de providers de scrape até um extrair um JSON válido.
func (ai *AiService) GenerateScrapeSite(ctx context.Context, description string) (*scraper.NLScraperResult, error) {
	var result *scraper.NLScraperResult
	err := ai.withFallback(ctx, PurposeScrape, func(ctx context.Context, p Provider, last bool) (Usage, error) {
		r, usage, err := ai.generateScrapeSite(ctx, p, last, description)
		result = r
		return usage, err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (ai *AiService) generateScrapeSite(ctx context.Context, p Provider, waitOn429 bool, description string) (*scraper.NLScraperResult, Usage, error) {
	// Timeout por provider: um modelo travado não consome o tempo dos próximos da cadeia.
	scrapeTimeout := config.GetConfigDurationOrDefault(ai.Config.ScrapeAi.Timeout, 3600*time.Second)
	scrapeCtx, cancel := context.WithTimeout(ctx, scrapeTimeout)
	defer cancel()

	body := ai.buildScrapeBody(description, p)
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, Usage{}, fmt.Errorf("AiService: error in generate a json for body: %w", err)
	}

	raw, err := ai.sendRequest(scrapeCtx, ai.scrapeHttpClient, payload, p, waitOn429)
	if err != nil {
		return nil, Usage{}, err
	}

	content, err := extractContent(raw)
	if err != nil {
		return nil, raw.Usage, err
	}
	cleanContent := sanitizeJSONLiterals(stripMarkdownCode(content))
	cleanContent = repairJSON(cleanContent)

	var result scraper.NLScraperResult
	if err := json.Unmarshal([]byte(cleanContent), &result); err != nil {
		return nil, raw.Usage, fmt.Errorf("AiService: %w: %v (início da resposta: %q)", ErrParse, err, snippet(cleanContent))
	}
	return &result, raw.Usage, nil
}

type LLMResponse struct {
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Usage são os tokens consumidos numa chamada (para as métricas).
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

type Choice struct {
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type geminiResponse struct {
	Candidates []struct {
		FinishReason string `json:"finishReason"`
		Content      struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
	} `json:"usageMetadata"`
}

// sendRequest envia o payload para um provider. Com waitOn429=false, um 429 retorna
// *ErrRateLimit na hora (há outro provider na cadeia); com true, espera e repete.
func (ai *AiService) sendRequest(ctx context.Context, client *http.Client, payload []byte, p Provider, waitOn429 bool) (LLMResponse, error) {
	const maxRetries = 3
	var (
		lastErr      error
		hitRateLimit bool
	)
	for attempt := range maxRetries {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return LLMResponse{}, ctx.Err()
			default:
			}
		}

		req, err := ai.buildRequest(ctx, payload, p)
		if err != nil {
			return LLMResponse{}, fmt.Errorf("AiService: failed to build request to ai: %w", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			// Erro de rede (timeout, conexão recusada etc.): retry com backoff exponencial.
			// Não sinaliza rate limit — o worker não deve pausar por falha de rede.
			lastErr = fmt.Errorf("AiService: request failed: %w", err)
			backoff := time.Duration(1<<uint(attempt)) * time.Second // 1s, 2s, 4s
			select {
			case <-ctx.Done():
				return LLMResponse{}, ctx.Err()
			case <-time.After(backoff):
			}
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			hitRateLimit = true
			lastErr = fmt.Errorf("AiService: status 429: %s", string(body))
			if !waitOn429 {
				break
			}
			delay := parseRetryDelay(body)
			select {
			case <-ctx.Done():
				return LLMResponse{}, ctx.Err()
			case <-time.After(delay):
			}
			continue
		}

		if resp.StatusCode != http.StatusOK {
			errBody, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return LLMResponse{}, &ErrHTTPStatus{Code: resp.StatusCode, Body: truncateUTF8(string(errBody), 2000)}
		}

		if p.Name == "gemini" {
			var gr geminiResponse
			err := json.NewDecoder(resp.Body).Decode(&gr)
			resp.Body.Close()
			if err != nil {
				return LLMResponse{}, fmt.Errorf("AiService: decode gemini: %w", err)
			}
			if len(gr.Candidates) == 0 || len(gr.Candidates[0].Content.Parts) == 0 {
				return LLMResponse{}, fmt.Errorf("AiService: %w (gemini sem candidates)", ErrEmpty)
			}
			return LLMResponse{Choices: []Choice{{
				Message: Message{
					Role:    "assistant",
					Content: gr.Candidates[0].Content.Parts[0].Text,
				},
				FinishReason: gr.Candidates[0].FinishReason,
			}}, Usage: Usage{
				PromptTokens:     gr.UsageMetadata.PromptTokenCount,
				CompletionTokens: gr.UsageMetadata.CandidatesTokenCount,
			}}, nil
		}

		var raw LLMResponse
		err = json.NewDecoder(resp.Body).Decode(&raw)
		resp.Body.Close()
		if err != nil {
			return LLMResponse{}, fmt.Errorf("AiService: decode: %w", err)
		}
		if len(raw.Choices) == 0 {
			return LLMResponse{}, fmt.Errorf("AiService: %w (sem choices)", ErrEmpty)
		}
		return raw, nil
	}

	// Só sinaliza ErrRateLimit (→ pausa o worker) quando realmente houve 429.
	// Erros de rede não devem travar o worker por 12h.
	if hitRateLimit {
		return LLMResponse{}, &ErrRateLimit{Msg: lastErr.Error()}
	}
	return LLMResponse{}, lastErr
}

func (ai *AiService) scrapSiteLLM(targetURL string) (string, error) {
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}
	scraper.ApplyHeaders(req)

	resp, err := scraper.DoRequestWithRetry(ai.httpClient, req, 3)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("site returned status: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", err
	}

	doc.Find("script, style, nav, footer, header, noscript").Each(func(i int, s *goquery.Selection) {
		s.Remove()
	})

	const maxChars = 8000
	content := scraper.CleanString(doc.Find("body").Text())
	if len(content) > maxChars {
		content = content[:maxChars]
	}
	return content, nil
}

// ExtractJSON percorre a cadeia do purpose pedindo um JSON e entrega o conteúdo limpo a
// decode. Se decode falhar (JSON inválido ou vazio para o caller), o próximo provider tenta.
// Temperatura baixa: aqui a IA só transcreve dados, não escreve texto novo.
func (ai *AiService) ExtractJSON(ctx context.Context, purpose Purpose, perProvider time.Duration, systemPrompt, userPrompt string, decode func(content string) error) error {
	return ai.withFallback(ctx, purpose, func(ctx context.Context, p Provider, last bool) (Usage, error) {
		// Um modelo travado não pode consumir o tempo dos próximos da cadeia.
		ctx, cancel := context.WithTimeout(ctx, perProvider)
		defer cancel()

		body := ai.buildBody(AIInput{SystemPrompt: systemPrompt, UserPrompt: userPrompt}, p)
		if gc, ok := body["generationConfig"].(map[string]any); ok {
			gc["temperature"] = 0.1
		} else {
			body["temperature"] = 0.1
		}
		payload, err := json.Marshal(body)
		if err != nil {
			return Usage{}, fmt.Errorf("AiService: error in generate a json for body: %w", err)
		}

		raw, err := ai.sendRequest(ctx, ai.httpClient, payload, p, last)
		if err != nil {
			return Usage{}, err
		}
		text, err := extractContent(raw)
		if err != nil {
			return raw.Usage, err
		}
		content := repairJSON(sanitizeJSONLiterals(stripMarkdownCode(text)))
		if err := decode(content); err != nil {
			return raw.Usage, fmt.Errorf("AiService: %w: %v", ErrParse, err)
		}
		return raw.Usage, nil
	})
}
