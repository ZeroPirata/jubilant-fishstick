package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"hackton-treino/config"
	"hackton-treino/internal/scraper"
	"io"
	"net/http"
	"time"

	"github.com/PuerkitoBio/goquery"
)

func NewLLMService(cfg *config.Config) *AiService {
	timeout := config.GetConfigDurationOrDefault(cfg.Ai.Timeout, 3600*time.Second)
	scrapeTimeout := config.GetConfigDurationOrDefault(cfg.ScrapeAi.Timeout, 120*time.Second)
	return &AiService{
		Config: cfg,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		scrapeHttpClient: &http.Client{
			Timeout: scrapeTimeout,
		},
	}
}

func (ai *AiService) buildRequest(ctx context.Context, payload []byte, isScrape bool) (*http.Request, error) {
	targetURL := ai.Config.Ai.Url
	key := ai.Config.Ai.Key
	provider := ai.Config.Ai.Provider

	if isScrape && ai.Config.ScrapeAi.Activate {
		targetURL = ai.Config.ScrapeAi.Url
		key = ai.Config.ScrapeAi.Key
		provider = ai.Config.ScrapeAi.Provider
	}

	if provider == "gemini" && key != "" {
		targetURL = targetURL + "?key=" + key
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("AiService: failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	if provider != "ollama" && provider != "gemini" && key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}

	return req, nil
}

type AIInput struct {
	SystemPrompt string
	UserPrompt   string
	Description  string
}

func (ai *AiService) buildBody(input AIInput) map[string]any {
	if input.Description != "" {
		scrapeProvider := ai.Config.Ai.Provider
		scrapeModel := ai.Config.Ai.Model
		if ai.Config.ScrapeAi.Activate {
			scrapeProvider = ai.Config.ScrapeAi.Provider
			scrapeModel = ai.Config.ScrapeAi.Model
		}

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

Stack inclui (somente se explicitamente citados): linguagens, frameworks, bancos, cloud, ferramentas, domínios técnicos, metodologias e boas práticas nomeadas.
Requirements inclui APENAS: anos de experiência, idiomas, formação acadêmica.

IDIOMA: extraia os termos como aparecem no texto. Não traduza nem normalize — a normalização é feita pelo sistema depois.

EXEMPLO PT-BR:
Input: "Dev Backend. 3 anos com Golang e Postgres. Conhecimento em mensageria e AWS. Inglês fluente."
Output: {"Description":"Desenvolvedor Backend com foco em mensageria e nuvem.","Stack":["Golang","PostgreSQL","Mensageria","AWS"],"Requirements":["3 anos de experiência","Inglês fluente"]}

EXEMPLO EN:
Input: "Experience with RESTful/gRPC APIs, PostgreSQL, Docker. Kafka, RabbitMQ, or NATS is a plus."
Output: {"Description":"Backend engineer with API and messaging experience.","Stack":["REST","gRPC","PostgreSQL","Docker","Kafka","RabbitMQ","NATS"],"Requirements":[]}`

		userMsg := "Analise o seguinte texto de vaga:\n\n" + input.Description

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
					"maxOutputTokens":  800,
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
					"num_predict": 800,
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
			"max_tokens":  800,
			"messages": []map[string]any{
				{"role": "system", "content": systemPrompt},
				{"role": "user", "content": userMsg},
			},
		}
	}

	if ai.Config.Ai.Provider == "gemini" {
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
				"maxOutputTokens":  8192,
			},
		}
	}

	body := map[string]any{
		"model": ai.Config.Ai.Model,
		"messages": []map[string]any{
			{"role": "system", "content": input.SystemPrompt},
			{"role": "user", "content": input.UserPrompt},
		},
	}

	return ai.applyJsonFormat(body, ai.Config.Ai.Provider)
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

func (ai *AiService) GenerateCurriculum(ctx context.Context, systemPrompt, userPrompt string) (*LLMCurriculoResponse, error) {
	body := ai.buildBody(AIInput{SystemPrompt: systemPrompt, UserPrompt: userPrompt})
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("AiService: error in generate a json for body: %w", err)
	}

	raw, err := ai.sendRequest(ctx, payload, false)
	if err != nil {
		return nil, err
	}

	content := sanitizeJSONLiterals(stripMarkdownCode(raw.Choices[0].Message.Content))
	var result LLMCurriculoResponse
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("AiService: parse llm json: %w", err)
	}
	if result.Error != "" {
		return nil, fmt.Errorf("AiService: llm error: %s", result.Error)
	}
	return &result, nil
}

func (ai *AiService) GenerateScrapeSite(ctx context.Context, description string) (*scraper.NLScraperResult, error) {
	scrapeTimeout := config.GetConfigDurationOrDefault(ai.Config.ScrapeAi.Timeout, 3600*time.Second)
	scrapeCtx, cancel := context.WithTimeout(ctx, scrapeTimeout)
	defer cancel()

	body := ai.buildBody(AIInput{Description: description})
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("AiService: error in generate a json for body: %w", err)
	}

	raw, err := ai.sendRequest(scrapeCtx, payload, true)
	if err != nil {
		return nil, err
	}

	content := raw.Choices[0].Message.Content
	cleanContent := sanitizeJSONLiterals(stripMarkdownCode(content))
	cleanContent = repairJSON(cleanContent)

	var result scraper.NLScraperResult
	if err := json.Unmarshal([]byte(cleanContent), &result); err != nil {
		return nil, fmt.Errorf("AiService: parse scraper llm json: %w", err)
	}
	return &result, nil
}

type LLMResponse struct {
	Choices []Choice `json:"choices"`
}

type Choice struct {
	Message Message `json:"message"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

func (ai *AiService) sendRequest(ctx context.Context, payload []byte, isScrape bool) (LLMResponse, error) {
	client := ai.httpClient
	if isScrape {
		client = ai.scrapeHttpClient
	}

	provider := ai.Config.Ai.Provider
	if isScrape && ai.Config.ScrapeAi.Activate {
		provider = ai.Config.ScrapeAi.Provider
	}

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

		req, err := ai.buildRequest(ctx, payload, isScrape)
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
			return LLMResponse{}, fmt.Errorf("AiService: status %d: %s", resp.StatusCode, string(errBody))
		}

		if provider == "gemini" {
			var gr geminiResponse
			err := json.NewDecoder(resp.Body).Decode(&gr)
			resp.Body.Close()
			if err != nil {
				return LLMResponse{}, fmt.Errorf("AiService: decode gemini: %w", err)
			}
			if len(gr.Candidates) == 0 || len(gr.Candidates[0].Content.Parts) == 0 {
				return LLMResponse{}, fmt.Errorf("AiService: empty gemini response")
			}
			return LLMResponse{Choices: []Choice{{Message: Message{
				Role:    "assistant",
				Content: gr.Candidates[0].Content.Parts[0].Text,
			}}}}, nil
		}

		var raw LLMResponse
		err = json.NewDecoder(resp.Body).Decode(&raw)
		resp.Body.Close()
		if err != nil {
			return LLMResponse{}, fmt.Errorf("AiService: decode: %w", err)
		}
		if len(raw.Choices) == 0 {
			return LLMResponse{}, fmt.Errorf("AiService: empty response")
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
