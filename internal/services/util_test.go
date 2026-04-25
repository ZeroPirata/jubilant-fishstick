package services

import (
	"testing"
	"time"
)

// TestStripMarkdownCode verifica a remoção de blocos ```json``` que a LLM às vezes
// adiciona ao redor do JSON mesmo quando pedimos JSON puro.
func TestStripMarkdownCode(t *testing.T) {
	cases := []struct {
		label string
		input string
		want  string
	}{
		{
			label: "sem markdown — retorna igual",
			input: `{"key":"value"}`,
			want:  `{"key":"value"}`,
		},
		{
			label: "bloco json com linguagem",
			input: "```json\n{\"key\":\"value\"}\n```",
			want:  `{"key":"value"}`,
		},
		{
			label: "bloco sem linguagem",
			input: "```\n{\"key\":\"value\"}\n```",
			want:  `{"key":"value"}`,
		},
		{
			label: "espacos extras nas bordas",
			input: "  ```json\n{\"key\":\"value\"}\n```  ",
			want:  `{"key":"value"}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			got := stripMarkdownCode(tc.input)
			if got != tc.want {
				t.Errorf("stripMarkdownCode(%q)\n got  %q\n want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestRepairJSON testa a recuperação de JSONs malformados que a LLM pode gerar.
func TestRepairJSON(t *testing.T) {
	cases := []struct {
		label string
		input string
		want  string
	}{
		{
			label: "json valido — passa sem alteracao",
			input: `{"a":1,"b":2}`,
			want:  `{"a":1,"b":2}`,
		},
		{
			label: "trailing comma em objeto",
			input: `{"a":1,"b":2,}`,
			want:  `{"a":1,"b":2}`,
		},
		{
			label: "trailing comma em array",
			input: `{"a":[1,2,3,]}`,
			want:  `{"a":[1,2,3]}`,
		},
		{
			label: "chave faltando fechamento",
			input: `{"a":1`,
			want:  `{"a":1}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			got := repairJSON(tc.input)
			if got != tc.want {
				t.Errorf("repairJSON(%q)\n got  %q\n want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestSanitizeJSONLiterals garante que newlines literais dentro de strings JSON
// são escapados — caso contrário json.Unmarshal falha com "invalid character".
func TestSanitizeJSONLiterals(t *testing.T) {
	cases := []struct {
		label string
		input string
		want  string
	}{
		{
			label: "json sem caracteres especiais — passa igual",
			input: `{"key":"value"}`,
			want:  `{"key":"value"}`,
		},
		{
			label: "newline literal dentro de string",
			input: "{\"key\":\"linha1\nlinha2\"}",
			want:  `{"key":"linha1\nlinha2"}`,
		},
		{
			label: "tab literal dentro de string",
			input: "{\"key\":\"col1\tcol2\"}",
			want:  `{"key":"col1\tcol2"}`,
		},
		{
			label: "newline fora de string nao e alterado",
			input: "{\n\"key\":\"value\"\n}",
			want:  "{\n\"key\":\"value\"\n}",
		},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			got := sanitizeJSONLiterals(tc.input)
			if got != tc.want {
				t.Errorf("sanitizeJSONLiterals(%q)\n got  %q\n want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestParseRetryDelay verifica a extração do retryDelay do body de erro 429 do Gemini
// e o fallback de 30s quando o body não tem o campo.
func TestParseRetryDelay(t *testing.T) {
	cases := []struct {
		label string
		body  []byte
		want  time.Duration
	}{
		{
			label: "body vazio — usa fallback 30s",
			body:  []byte(`{}`),
			want:  30 * time.Second,
		},
		{
			label: "json malformado — usa fallback 30s",
			body:  []byte(`not json`),
			want:  30 * time.Second,
		},
		{
			label: "retryDelay presente no formato Gemini — adiciona 2s de margem",
			body: []byte(`{
				"error": {
					"details": [{
						"@type": "type.googleapis.com/google.rpc.RetryInfo",
						"retryDelay": "5s"
					}]
				}
			}`),
			want: 7 * time.Second, // 5s + 2s de margem
		},
		{
			label: "retryDelay com decimais",
			body: []byte(`{
				"error": {
					"details": [{
						"@type": "type.googleapis.com/google.rpc.RetryInfo",
						"retryDelay": "17.356s"
					}]
				}
			}`),
			// 17.356s + 2s = 19.356s — testamos só que é > 19s para evitar float comparison
			want: 0, // sentinel: veja assert abaixo
		},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			got := parseRetryDelay(tc.body)

			// caso especial: retryDelay com decimais — só verifica o range
			if tc.want == 0 {
				const min = 19 * time.Second
				const max = 20 * time.Second
				if got < min || got > max {
					t.Errorf("parseRetryDelay com decimais = %v, want entre %v e %v", got, min, max)
				}
				return
			}

			if got != tc.want {
				t.Errorf("parseRetryDelay() = %v, want %v", got, tc.want)
			}
		})
	}
}
