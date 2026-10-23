package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.uber.org/zap"
)

type Purpose string

const (
	PurposeMain   Purpose = "main"   // geração do currículo
	PurposeScrape Purpose = "scrape" // extração dos dados da vaga
)

// Provider é uma entrada da cadeia de fallback.
type Provider struct {
	Name  string // "gemini" | "ollama" | qualquer outro é tratado como OpenAI-compatible
	URL   string
	Key   string
	Model string
}

func (p Provider) Label() string {
	if p.Model == "" {
		return p.Name
	}
	return p.Name + "/" + p.Model
}

// ProviderSource devolve a cadeia configurada no painel. Lista vazia = usar o .env.
type ProviderSource interface {
	Providers(ctx context.Context, purpose Purpose) ([]Provider, error)
}

// resolveProviders monta a cadeia na ordem de prioridade:
// painel (banco) → .env do purpose → (scrape apenas) cadeia principal.
func (ai *AiService) resolveProviders(ctx context.Context, purpose Purpose) []Provider {
	if ai.Source != nil {
		ps, err := ai.Source.Providers(ctx, purpose)
		if err != nil {
			zap.L().Warn("AiService: falha ao carregar providers do painel, usando .env",
				zap.String("purpose", string(purpose)), zap.Error(err))
		} else if len(ps) > 0 {
			return ps
		}
	}

	if purpose == PurposeScrape {
		if ai.Config.ScrapeAi.Activate {
			return []Provider{{
				Name:  ai.Config.ScrapeAi.Provider,
				URL:   ai.Config.ScrapeAi.Url,
				Key:   ai.Config.ScrapeAi.Key,
				Model: ai.Config.ScrapeAi.Model,
			}}
		}
		return ai.resolveProviders(ctx, PurposeMain)
	}

	return []Provider{{
		Name:  ai.Config.Ai.Provider,
		URL:   ai.Config.Ai.Url,
		Key:   ai.Config.Ai.Key,
		Model: ai.Config.Ai.Model,
	}}
}

// withFallback chama call para cada provider da cadeia, em ordem, até um ter sucesso.
// last indica o último provider: só ele espera e repete em caso de 429, os anteriores
// passam a vez imediatamente. Se todos falharem por rate limit, retorna *ErrRateLimit
// (o worker pausa); se algum falhou por outro motivo, retorna um erro comum.
func (ai *AiService) withFallback(ctx context.Context, purpose Purpose, call func(ctx context.Context, p Provider, last bool) error) error {
	providers := ai.resolveProviders(ctx, purpose)
	if len(providers) == 0 {
		return fmt.Errorf("AiService: nenhum provider de IA configurado para %q", purpose)
	}

	var (
		msgs           []string
		allRateLimited = true
	)
	for i, p := range providers {
		if err := ctx.Err(); err != nil {
			return err
		}

		err := call(ctx, p, i == len(providers)-1)
		if err == nil {
			if i > 0 {
				zap.L().Info("AiService: fallback usado",
					zap.String("purpose", string(purpose)), zap.String("provider", p.Label()), zap.Int("posicao", i+1))
			}
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}

		var errRL *ErrRateLimit
		if !errors.As(err, &errRL) {
			allRateLimited = false
		}
		msgs = append(msgs, fmt.Sprintf("[%d] %s: %v", i+1, p.Label(), err))
		if i < len(providers)-1 {
			zap.L().Warn("AiService: provider falhou, tentando o próximo",
				zap.String("purpose", string(purpose)), zap.String("provider", p.Label()), zap.Error(err))
		}
	}

	msg := strings.Join(msgs, " | ")
	if allRateLimited {
		return &ErrRateLimit{Msg: msg}
	}
	return errors.New(msg)
}
