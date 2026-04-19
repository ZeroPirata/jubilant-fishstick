package services

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"
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

// repairJSON remove trailing commas and closes any unclosed brackets.
func repairJSON(s string) string {
	s = strings.TrimSpace(s)
	s = reTrailingComma.ReplaceAllString(s, "$1")
	open := 0
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
		case '{', '[':
			open++
		case '}', ']':
			open--
		}
	}
	for range open {
		s += "}"
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
