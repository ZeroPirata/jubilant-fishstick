package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"hackton-treino/config"
	"io"
	"net/http"
	"time"
)

func NewAiService(cfg *config.Config) *AiService {
	return &AiService{
		Config: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (ai *AiService) generateRequest(ctx context.Context, payload []byte) (*http.Request, error) {
	url := ai.Config.Ai.Url
	key := "Bearer " + ai.Config.Ai.Key

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", key)

	return req, nil
}

func (ai *AiService) GerarCurriculo(ctx context.Context, systemPrompt, userPrompt string) (*LLMResponse, error) {
	body := map[string]any{
		"model": ai.Config.Ai.Model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
	}

	payload, _ := json.Marshal(body)
	req, err := ai.generateRequest(ctx, payload)
	if err != nil {
		return nil, err
	}

	resp, err := ai.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("AiService: request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("AiService: status %d: %s", resp.StatusCode, string(errBody))
	}

	var raw struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("grok: decode: %w", err)
	}
	if len(raw.Choices) == 0 {
		return nil, fmt.Errorf("grok: empty response")
	}

	content := sanitizeJSONLiterals(stripMarkdownCode(raw.Choices[0].Message.Content))

	var result LLMResponse
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("grok: parse llm json: %w", err)
	}
	if result.Error != "" {
		return nil, fmt.Errorf("grok: llm error: %s", result.Error)
	}
	return &result, nil
}
