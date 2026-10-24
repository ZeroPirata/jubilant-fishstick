// Package aiconfig guarda a cadeia de providers de IA editável pelo painel master.
// Implementa services.ProviderSource: o AiService lê daqui a cada chamada (com cache curto),
// então uma alteração salva no painel vale para o próximo job, sem reiniciar a aplicação.
package aiconfig

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"

	"hackton-treino/internal/repository/aisettings"
	"hackton-treino/internal/security"
	"hackton-treino/internal/services"

	"go.uber.org/zap"
)

const (
	cacheTTL     = 30 * time.Second
	maxProviders = 10
)

// Providers aceitos. "openai" cobre qualquer API OpenAI-compatible (Groq, OpenRouter, NVIDIA...).
var knownProviders = map[string]bool{"gemini": true, "ollama": true, "openai": true}

// ValidationError indica entrada inválida do painel (vira HTTP 400).
type ValidationError struct{ Msg string }

func (e *ValidationError) Error() string { return e.Msg }

// View é o que o painel recebe: a key nunca sai em texto puro.
type View struct {
	ID        string    `json:"id"`
	Provider  string    `json:"provider"`
	URL       string    `json:"url"`
	Model     string    `json:"model"`
	Enabled   bool      `json:"enabled"`
	HasKey    bool      `json:"has_key"`
	KeyHint   string    `json:"key_hint,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Input é uma entrada enviada pelo painel. APIKey vazia mantém a key já salva
// para aquele ID; ClearKey remove.
type Input struct {
	ID       string `json:"id"`
	Provider string `json:"provider"`
	URL      string `json:"url"`
	Model    string `json:"model"`
	APIKey   string `json:"api_key"`
	ClearKey bool   `json:"clear_key"`
	// KeyFrom: ID de um provider já salvo (de qualquer cadeia) cuja key será copiada.
	// É como o painel coloca na outra cadeia uma IA que já tem key, sem digitá-la de novo.
	KeyFrom string `json:"key_from"`
	Enabled bool   `json:"enabled"`
}

type cached struct {
	providers []services.Provider
	at        time.Time
}

type Store struct {
	repo  aisettings.Repository
	box   *security.SecretBox
	mu    sync.RWMutex
	cache map[services.Purpose]cached
}

func NewStore(repo aisettings.Repository, box *security.SecretBox) *Store {
	return &Store{repo: repo, box: box, cache: map[services.Purpose]cached{}}
}

func ParsePurpose(s string) (services.Purpose, bool) {
	p := services.Purpose(s)
	return p, p == services.PurposeMain || p == services.PurposeScrape
}

// EncryptionEnabled informa se dá para salvar keys (SETTINGS_ENCRYPTION_KEY configurada).
func (s *Store) EncryptionEnabled() bool { return s.box != nil }

// Providers devolve a cadeia ativa (só habilitados, keys descriptografadas).
func (s *Store) Providers(ctx context.Context, purpose services.Purpose) ([]services.Provider, error) {
	s.mu.RLock()
	c, ok := s.cache[purpose]
	s.mu.RUnlock()
	if ok && time.Since(c.at) < cacheTTL {
		return c.providers, nil
	}

	rows, err := s.repo.List(ctx, string(purpose))
	if err != nil {
		return nil, err
	}

	var out []services.Provider
	for _, r := range rows {
		if !r.Enabled {
			continue
		}
		p := services.Provider{Name: r.Provider, URL: r.URL, Model: r.Model}
		if len(r.APIKeyEnc) > 0 {
			key, err := s.box.Open(r.APIKeyEnc)
			if err != nil {
				// Pula só este provider: os outros da cadeia continuam funcionando.
				zap.L().Error("aiconfig: não foi possível ler a API key, provider ignorado",
					zap.String("purpose", string(purpose)), zap.String("provider", p.Label()), zap.Error(err))
				continue
			}
			p.Key = key
		}
		out = append(out, p)
	}

	s.mu.Lock()
	s.cache[purpose] = cached{providers: out, at: time.Now()}
	s.mu.Unlock()
	return out, nil
}

// List devolve a cadeia completa (inclusive desabilitados) para o painel.
func (s *Store) List(ctx context.Context, purpose services.Purpose) ([]View, error) {
	rows, err := s.repo.List(ctx, string(purpose))
	if err != nil {
		return nil, err
	}
	out := make([]View, 0, len(rows))
	for _, r := range rows {
		v := View{
			ID: r.ID, Provider: r.Provider, URL: r.URL, Model: r.Model,
			Enabled: r.Enabled, HasKey: len(r.APIKeyEnc) > 0, UpdatedAt: r.UpdatedAt,
		}
		if v.HasKey {
			if key, err := s.box.Open(r.APIKeyEnc); err == nil {
				v.KeyHint = maskKey(key)
			} else {
				v.KeyHint = "ilegível — salve a key de novo"
			}
		}
		out = append(out, v)
	}
	return out, nil
}

// Save troca a cadeia inteira do purpose e invalida o cache.
func (s *Store) Save(ctx context.Context, purpose services.Purpose, inputs []Input, userID string) error {
	return s.SaveAll(ctx, map[services.Purpose][]Input{purpose: inputs}, userID)
}

// SaveAll troca as cadeias recebidas numa transação só (o painel salva principal e
// scrape juntos: uma IA pode estar nas duas).
func (s *Store) SaveAll(ctx context.Context, chains map[services.Purpose][]Input, userID string) error {
	keysByPurpose := map[services.Purpose]map[string][]byte{}
	allKeys := map[string][]byte{}
	for _, purpose := range []services.Purpose{services.PurposeMain, services.PurposeScrape} {
		existing, err := s.repo.List(ctx, string(purpose))
		if err != nil {
			return err
		}
		keys := make(map[string][]byte, len(existing))
		for _, r := range existing {
			keys[r.ID] = r.APIKeyEnc
			allKeys[r.ID] = r.APIKeyEnc
		}
		keysByPurpose[purpose] = keys
	}

	out := make(map[string][]aisettings.ProviderRow, len(chains))
	for purpose, inputs := range chains {
		if _, ok := ParsePurpose(string(purpose)); !ok {
			return &ValidationError{fmt.Sprintf("cadeia %q inválida", purpose)}
		}
		rows, err := s.buildRows(purpose, inputs, keysByPurpose[purpose], allKeys)
		if err != nil {
			return err
		}
		out[string(purpose)] = rows
	}

	if err := s.repo.Replace(ctx, out, userID); err != nil {
		return err
	}
	for purpose := range chains {
		s.Invalidate(purpose)
	}
	return nil
}

func (s *Store) buildRows(purpose services.Purpose, inputs []Input, own, all map[string][]byte) ([]aisettings.ProviderRow, error) {
	if len(inputs) > maxProviders {
		return nil, &ValidationError{fmt.Sprintf("no máximo %d providers por cadeia", maxProviders)}
	}
	name := map[services.Purpose]string{services.PurposeMain: "principal", services.PurposeScrape: "scrape"}[purpose]

	rows := make([]aisettings.ProviderRow, 0, len(inputs))
	for i, in := range inputs {
		if err := validate(in); err != nil {
			return nil, &ValidationError{fmt.Sprintf("%s, provider %d: %s", name, i+1, err)}
		}

		row := aisettings.ProviderRow{
			Provider: in.Provider, URL: in.URL, Model: in.Model, Enabled: in.Enabled,
		}
		// Só reaproveita ID (e a key guardada nele) se ele pertence a esta cadeia.
		if _, ok := own[in.ID]; ok {
			row.ID = in.ID
		}

		switch {
		case in.ClearKey:
		case in.APIKey != "":
			sealed, err := s.box.Seal(in.APIKey)
			if errors.Is(err, security.ErrSecretBoxDisabled) {
				return nil, &ValidationError{"SETTINGS_ENCRYPTION_KEY não está configurada no servidor; não é possível salvar API keys"}
			}
			if err != nil {
				return nil, err
			}
			row.APIKeyEnc = sealed
		case row.ID != "":
			row.APIKeyEnc = own[row.ID]
		case in.KeyFrom != "":
			row.APIKeyEnc = all[in.KeyFrom]
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func (s *Store) Invalidate(purpose services.Purpose) {
	s.mu.Lock()
	delete(s.cache, purpose)
	s.mu.Unlock()
}

func validate(in Input) error {
	if !knownProviders[in.Provider] {
		return fmt.Errorf("provider %q inválido (use gemini, ollama ou openai)", in.Provider)
	}
	u, err := url.Parse(in.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("url %q inválida", in.URL)
	}
	// No Gemini o modelo faz parte da URL; nos outros vai no body e é obrigatório.
	if in.Model == "" && in.Provider != "gemini" {
		return errors.New("modelo é obrigatório")
	}
	return nil
}

func maskKey(key string) string {
	if len(key) < 12 {
		return "••••"
	}
	return "••••" + key[len(key)-4:]
}
