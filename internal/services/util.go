package services

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// parseRetryDelay extrai o retryDelay do body de erro 429 do Gemini.
// Retorna 30s como fallback se não encontrar.
func parseRetryDelay(body []byte) time.Duration {
	const fallback = 30 * time.Second
	var errResp struct {
		Error struct {
			Details []struct {
				Type       string `json:"@type"`
				RetryDelay string `json:"retryDelay"`
			} `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &errResp); err != nil {
		return fallback
	}
	for _, d := range errResp.Error.Details {
		if d.Type == "type.googleapis.com/google.rpc.RetryInfo" && d.RetryDelay != "" {
			// formato: "17s" ou "17.356876412s"
			d, err := time.ParseDuration(d.RetryDelay)
			if err == nil {
				return d + 2*time.Second // margem de segurança
			}
		}
	}
	return fallback
}

var reTrailingComma = regexp.MustCompile(`,\s*([\]\}])`)

// Modelos de raciocínio (DeepSeek-R1, Qwen3...) podem mandar o "pensamento" dentro do content.
var reThinkBlock = regexp.MustCompile(`(?s)<think>.*?</think>`)

// extractContent pega o texto útil da resposta e transforma os casos silenciosos
// (cortada por limite de tokens, vazia) em erros claros — que acionam o fallback.
func extractContent(raw LLMResponse) (string, error) {
	choice := raw.Choices[0]
	content := strings.TrimSpace(reThinkBlock.ReplaceAllString(choice.Message.Content, ""))

	switch choice.FinishReason {
	case "length", "MAX_TOKENS":
		return "", fmt.Errorf("AiService: %w (finish_reason=%s, %d caracteres recebidos)",
			ErrTruncated, choice.FinishReason, len(content))
	}
	if content == "" {
		return "", fmt.Errorf("AiService: %w (finish_reason=%q); modelos de raciocínio podem gastar o limite pensando", ErrEmpty, choice.FinishReason)
	}
	return content, nil
}

// snippet devolve o começo de s para mensagens de erro, sem estourar o log.
func snippet(s string) string { return truncateUTF8(s, 200) }

// truncateUTF8 corta s em até max bytes sem partir um caractere no meio.
func truncateUTF8(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := max
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "…"
}

// repairJSON remove trailing commas e fecha strings, arrays e objetos deixados abertos
// (resposta cortada), na ordem inversa em que foram abertos.
func repairJSON(s string) string {
	s = strings.TrimSpace(s)
	s = reTrailingComma.ReplaceAllString(s, "$1")
	var stack []rune
	inString := false
	escaped := false
	for _, c := range s {
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' {
			escaped = true
			continue
		}
		if c == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		switch c {
		case '{':
			stack = append(stack, '}')
		case '[':
			stack = append(stack, ']')
		case '}', ']':
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	if inString {
		if escaped {
			s = s[:len(s)-1]
		}
		s += `"`
	}
	for i := len(stack) - 1; i >= 0; i-- {
		s += string(stack[i])
	}
	return s
}

func stripMarkdownCode(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		s = s[strings.Index(s, "\n")+1:]
	}
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

func sanitizeJSONLiterals(s string) string {
	var buf strings.Builder
	buf.Grow(len(s))
	inString := false
	escaped := false
	for _, c := range s {
		if escaped {
			buf.WriteRune(c)
			escaped = false
			continue
		}
		if c == '\\' {
			buf.WriteRune(c)
			escaped = true
			continue
		}
		if c == '"' {
			inString = !inString
			buf.WriteRune(c)
			continue
		}
		if inString {
			switch c {
			case '\n':
				buf.WriteString(`\n`)
			case '\r':
				buf.WriteString(`\r`)
			case '\t':
				buf.WriteString(`\t`)
			default:
				buf.WriteRune(c)
			}
			continue
		}
		buf.WriteRune(c)
	}
	return buf.String()
}
