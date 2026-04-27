package util

import (
	"testing"
	"time"
)

// =============================================================================
// NormalizeStack
// =============================================================================

func TestNormalizeStack_DeduplicaEntradas(t *testing.T) {
	// Mesmo item em capitalizações diferentes deve aparecer só uma vez
	input := []string{"Go", "go", "GO", "PostgreSQL", "postgresql"}
	got := NormalizeStack(input, nil)

	if len(got) != 2 {
		t.Errorf("NormalizeStack: len = %d, want 2 (go, postgresql); got %v", len(got), got)
	}
}

func TestNormalizeStack_AplicaAlias(t *testing.T) {
	aliases := map[string]string{
		"golang":     "go",
		"k8s":        "kubernetes",
		"postgresql": "postgres",
	}
	input := []string{"Golang", "K8s", "PostgreSQL"}
	got := NormalizeStack(input, aliases)

	want := map[string]bool{"go": true, "kubernetes": true, "postgres": true}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d; got %v", len(got), len(want), got)
	}
	for _, item := range got {
		if !want[item] {
			t.Errorf("item inesperado: %q", item)
		}
	}
}

func TestNormalizeStack_RemoveVazios(t *testing.T) {
	input := []string{"Go", "", "  ", "PostgreSQL"}
	got := NormalizeStack(input, nil)

	// "" e "  " → TrimSpace → "" → ignorados
	if len(got) != 2 {
		t.Errorf("len = %d, want 2; got %v", len(got), got)
	}
}

func TestNormalizeStack_EntradaVazia(t *testing.T) {
	got := NormalizeStack(nil, nil)
	if len(got) != 0 {
		t.Errorf("NormalizeStack(nil) = %v, want []", got)
	}
}

func TestNormalizeStack_AliasDeduplicaComNativo(t *testing.T) {
	// "golang" → alias "go", mas "Go" já existe na lista.
	// O resultado deve conter "go" só uma vez.
	aliases := map[string]string{"golang": "go"}
	input := []string{"Go", "Golang"}
	got := NormalizeStack(input, aliases)

	if len(got) != 1 || got[0] != "go" {
		t.Errorf("NormalizeStack: got %v, want [go]", got)
	}
}

// =============================================================================
// ParsePgDate
// =============================================================================

func TestParsePgDate(t *testing.T) {
	cases := []struct {
		label     string
		input     string
		wantValid bool
		wantYear  int
		wantMonth time.Month
	}{
		{
			label:     "data valida ISO",
			input:     "2024-06-15",
			wantValid: true,
			wantYear:  2024,
			wantMonth: time.June,
		},
		{
			label:     "string vazia → invalid",
			input:     "",
			wantValid: false,
		},
		{
			label:     "formato errado → invalid",
			input:     "15/06/2024",
			wantValid: false,
		},
		{
			label:     "data parcial YYYY-MM → valid (formato do input[type=month])",
			input:     "2024-06",
			wantValid: true,
			wantYear:  2024,
			wantMonth: time.June,
		},
		{
			label:     "primeiro dia do ano",
			input:     "2000-01-01",
			wantValid: true,
			wantYear:  2000,
			wantMonth: time.January,
		},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			got := ParsePgDate(tc.input)

			if got.Valid != tc.wantValid {
				t.Fatalf("ParsePgDate(%q).Valid = %v, want %v", tc.input, got.Valid, tc.wantValid)
			}
			if tc.wantValid {
				if got.Time.Year() != tc.wantYear {
					t.Errorf("Year = %d, want %d", got.Time.Year(), tc.wantYear)
				}
				if got.Time.Month() != tc.wantMonth {
					t.Errorf("Month = %v, want %v", got.Time.Month(), tc.wantMonth)
				}
			}
		})
	}
}

// =============================================================================
// ConvertToPgText
// =============================================================================

func TestConvertToPgText(t *testing.T) {
	cases := []struct {
		label     string
		input     string
		wantValid bool
		wantStr   string
	}{
		{"string com valor", "hello", true, "hello"},
		{"string vazia → Invalid", "", false, ""},
		{"string com espacos", "  com espacos  ", true, "  com espacos  "},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			got := ConvertToPgText(tc.input)
			if got.Valid != tc.wantValid {
				t.Errorf("Valid = %v, want %v", got.Valid, tc.wantValid)
			}
			if got.String != tc.wantStr {
				t.Errorf("String = %q, want %q", got.String, tc.wantStr)
			}
		})
	}
}

func TestConvertToPgTextPtr(t *testing.T) {
	s := "valor"
	empty := ""

	cases := []struct {
		label     string
		input     *string
		wantValid bool
		wantStr   string
	}{
		{"nil → Invalid", nil, false, ""},
		{"ponteiro para vazio → Invalid", &empty, false, ""},
		{"ponteiro para valor", &s, true, "valor"},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			got := ConvertToPgTextPtr(tc.input)
			if got.Valid != tc.wantValid {
				t.Errorf("Valid = %v, want %v", got.Valid, tc.wantValid)
			}
			if got.String != tc.wantStr {
				t.Errorf("String = %q, want %q", got.String, tc.wantStr)
			}
		})
	}
}

// =============================================================================
// ParseUUID
// =============================================================================

func TestParseUUID(t *testing.T) {
	cases := []struct {
		label   string
		input   string
		wantErr bool
	}{
		{"UUID v4 valido", "550e8400-e29b-41d4-a716-446655440000", false},
		{"UUID nil", "00000000-0000-0000-0000-000000000000", false},
		{"string vazia", "", true},
		// pgtype.UUID.Scan aceita UUID sem hífens — comportamento da biblioteca, não bug
		{"uuid sem hifens aceito pelo pgtype", "550e8400e29b41d4a716446655440000", false},
		{"lixo aleatorio", "nao-sou-um-uuid", true},
		{"uuid truncado", "550e8400-e29b-41d4", true},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			_, err := ParseUUID(tc.input)
			if (err != nil) != tc.wantErr {
				t.Errorf("ParseUUID(%q): err = %v, wantErr = %v", tc.input, err, tc.wantErr)
			}
		})
	}
}

// =============================================================================
// SafeStringSlice / SafeString
// =============================================================================

func TestSafeStringSlice(t *testing.T) {
	t.Run("nil → nil", func(t *testing.T) {
		got := SafeStringSlice(nil)
		if got != nil {
			t.Errorf("SafeStringSlice(nil) = %v, want nil", got)
		}
	})

	t.Run("slice com valores", func(t *testing.T) {
		s := []string{"a", "b"}
		got := SafeStringSlice(&s)
		if len(got) != 2 || got[0] != "a" {
			t.Errorf("SafeStringSlice = %v, want [a b]", got)
		}
	})
}

func TestSafeString(t *testing.T) {
	t.Run("nil → vazio", func(t *testing.T) {
		if got := SafeString(nil); got != "" {
			t.Errorf("SafeString(nil) = %q, want \"\"", got)
		}
	})

	t.Run("ponteiro → valor", func(t *testing.T) {
		s := "hello"
		if got := SafeString(&s); got != "hello" {
			t.Errorf("SafeString = %q, want \"hello\"", got)
		}
	})
}

// =============================================================================
// PgTimeToPtr
// =============================================================================

func TestPgTimeToPtr(t *testing.T) {
	t.Run("zero time → nil", func(t *testing.T) {
		if got := PgTimeToPtr(time.Time{}); got != nil {
			t.Errorf("PgTimeToPtr(zero) = %v, want nil", got)
		}
	})

	t.Run("ano 1 → nil (sentinel de data vazia)", func(t *testing.T) {
		t1 := time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)
		if got := PgTimeToPtr(t1); got != nil {
			t.Errorf("PgTimeToPtr(year=1) = %v, want nil", got)
		}
	})

	t.Run("data valida → ponteiro nao nil", func(t *testing.T) {
		now := time.Now()
		got := PgTimeToPtr(now)
		if got == nil {
			t.Fatal("PgTimeToPtr(now) = nil, want non-nil")
		}
		if !got.Equal(now) {
			t.Errorf("*PgTimeToPtr = %v, want %v", *got, now)
		}
	})
}
