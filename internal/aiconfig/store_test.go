package aiconfig

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"hackton-treino/internal/repository/aisettings"
	"hackton-treino/internal/security"
	"hackton-treino/internal/services"
)

// memRepo guarda as cadeias em memória e conta as leituras (para testar o cache).
type memRepo struct {
	rows  map[string][]aisettings.ProviderRow
	lists int
	next  int
}

func newMemRepo() *memRepo { return &memRepo{rows: map[string][]aisettings.ProviderRow{}} }

func (m *memRepo) List(_ context.Context, purpose string) ([]aisettings.ProviderRow, error) {
	m.lists++
	return append([]aisettings.ProviderRow(nil), m.rows[purpose]...), nil
}

func (m *memRepo) Replace(_ context.Context, purpose string, rows []aisettings.ProviderRow, _ string) error {
	for i := range rows {
		if rows[i].ID == "" {
			m.next++
			rows[i].ID = "id-" + string(rune('0'+m.next))
		}
		rows[i].Purpose = purpose
		rows[i].Position = i
	}
	m.rows[purpose] = rows
	return nil
}

func newBox(t *testing.T) *security.SecretBox {
	t.Helper()
	box, err := security.NewSecretBox(base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32))))
	if err != nil {
		t.Fatal(err)
	}
	return box
}

var ctx = context.Background()

func TestStore_SalvaELeKeyCriptografada(t *testing.T) {
	repo := newMemRepo()
	s := NewStore(repo, newBox(t))

	err := s.Save(ctx, services.PurposeScrape, []Input{
		{Provider: "openai", URL: "https://api.groq.com/openai/v1/chat/completions", Model: "llama", APIKey: "gsk_1234567890abcd", Enabled: true},
	}, "user")
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	if strings.Contains(string(repo.rows["scrape"][0].APIKeyEnc), "gsk_") {
		t.Fatal("key foi salva em texto puro")
	}

	ps, _ := s.Providers(ctx, services.PurposeScrape)
	if len(ps) != 1 || ps[0].Key != "gsk_1234567890abcd" {
		t.Fatalf("Providers = %+v", ps)
	}

	views, _ := s.List(ctx, services.PurposeScrape)
	if !views[0].HasKey || views[0].KeyHint != "••••abcd" {
		t.Errorf("view = %+v, want has_key e hint ••••abcd", views[0])
	}
}

func TestStore_KeyVaziaMantemAAnterior(t *testing.T) {
	repo := newMemRepo()
	s := NewStore(repo, newBox(t))
	_ = s.Save(ctx, services.PurposeMain, []Input{{Provider: "openai", URL: "https://a", Model: "m", APIKey: "segredo-antigo-1234", Enabled: true}}, "u")
	id := repo.rows["main"][0].ID

	// Painel reenviou sem key (usuário só trocou o modelo) e adicionou um segundo provider.
	err := s.Save(ctx, services.PurposeMain, []Input{
		{ID: id, Provider: "openai", URL: "https://a", Model: "m2", Enabled: true},
		{Provider: "ollama", URL: "http://b", Model: "phi4", Enabled: true},
	}, "u")
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	ps, _ := s.Providers(ctx, services.PurposeMain)
	if len(ps) != 2 || ps[0].Key != "segredo-antigo-1234" || ps[0].Model != "m2" || ps[1].Key != "" {
		t.Errorf("Providers = %+v", ps)
	}
}

func TestStore_ClearKeyRemove(t *testing.T) {
	repo := newMemRepo()
	s := NewStore(repo, newBox(t))
	_ = s.Save(ctx, services.PurposeMain, []Input{{Provider: "openai", URL: "https://a", Model: "m", APIKey: "segredo-1234567", Enabled: true}}, "u")
	id := repo.rows["main"][0].ID

	_ = s.Save(ctx, services.PurposeMain, []Input{{ID: id, Provider: "openai", URL: "https://a", Model: "m", ClearKey: true, Enabled: true}}, "u")

	if len(repo.rows["main"][0].APIKeyEnc) != 0 {
		t.Error("key deveria ter sido removida")
	}
}

// Um ID de outra cadeia não pode "puxar" a key dela.
func TestStore_IDDeOutraCadeiaNaoReaproveitaKey(t *testing.T) {
	repo := newMemRepo()
	s := NewStore(repo, newBox(t))
	_ = s.Save(ctx, services.PurposeMain, []Input{{Provider: "openai", URL: "https://a", Model: "m", APIKey: "key-da-main-1234", Enabled: true}}, "u")
	mainID := repo.rows["main"][0].ID

	_ = s.Save(ctx, services.PurposeScrape, []Input{{ID: mainID, Provider: "openai", URL: "https://a", Model: "m", Enabled: true}}, "u")

	if len(repo.rows["scrape"][0].APIKeyEnc) != 0 {
		t.Error("scrape herdou a key da cadeia main")
	}
}

func TestStore_DesabilitadoNaoEntraNaCadeia(t *testing.T) {
	s := NewStore(newMemRepo(), newBox(t))
	_ = s.Save(ctx, services.PurposeMain, []Input{
		{Provider: "openai", URL: "https://a", Model: "off", Enabled: false},
		{Provider: "openai", URL: "https://b", Model: "on", Enabled: true},
	}, "u")

	ps, _ := s.Providers(ctx, services.PurposeMain)
	if len(ps) != 1 || ps[0].Model != "on" {
		t.Errorf("Providers = %+v, want só o habilitado", ps)
	}
}

func TestStore_CacheEInvalidacao(t *testing.T) {
	repo := newMemRepo()
	s := NewStore(repo, newBox(t))

	_, _ = s.Providers(ctx, services.PurposeMain)
	_, _ = s.Providers(ctx, services.PurposeMain)
	if repo.lists != 1 {
		t.Errorf("leituras no banco = %d, want 1 (segunda chamada deveria vir do cache)", repo.lists)
	}

	_ = s.Save(ctx, services.PurposeMain, []Input{{Provider: "openai", URL: "https://a", Model: "novo", Enabled: true}}, "u")
	ps, _ := s.Providers(ctx, services.PurposeMain)
	if len(ps) != 1 || ps[0].Model != "novo" {
		t.Errorf("após Save, Providers = %+v, want o modelo novo (cache invalidado)", ps)
	}
}

func TestStore_Validacao(t *testing.T) {
	cases := []struct {
		label string
		in    Input
	}{
		{"provider desconhecido", Input{Provider: "xpto", URL: "https://a", Model: "m"}},
		{"url sem esquema", Input{Provider: "openai", URL: "api.groq.com", Model: "m"}},
		{"url ftp", Input{Provider: "openai", URL: "ftp://a", Model: "m"}},
		{"sem modelo", Input{Provider: "openai", URL: "https://a"}},
	}
	s := NewStore(newMemRepo(), newBox(t))
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			err := s.Save(ctx, services.PurposeMain, []Input{tc.in}, "u")
			var verr *ValidationError
			if !errors.As(err, &verr) {
				t.Errorf("esperava ValidationError, recebeu %v", err)
			}
		})
	}

	// Gemini leva o modelo na URL, então modelo vazio é aceito.
	if err := s.Save(ctx, services.PurposeMain, []Input{{Provider: "gemini", URL: "https://g"}}, "u"); err != nil {
		t.Errorf("gemini sem modelo deveria ser aceito: %v", err)
	}
}

func TestStore_SemChaveDeCriptografia(t *testing.T) {
	s := NewStore(newMemRepo(), nil)

	err := s.Save(ctx, services.PurposeMain, []Input{{Provider: "openai", URL: "https://a", Model: "m", APIKey: "x"}}, "u")
	var verr *ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("esperava ValidationError sem SETTINGS_ENCRYPTION_KEY, recebeu %v", err)
	}

	// Sem key (ex.: ollama local) continua funcionando.
	if err := s.Save(ctx, services.PurposeMain, []Input{{Provider: "ollama", URL: "http://a", Model: "m", Enabled: true}}, "u"); err != nil {
		t.Errorf("salvar sem key deveria funcionar: %v", err)
	}
}
