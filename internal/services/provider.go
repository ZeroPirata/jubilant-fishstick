package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
)

type Purpose string

const (
	PurposeMain   Purpose = "main"   // geração do currículo
	PurposeScrape Purpose = "scrape" // extração dos dados da vaga
	PurposeImport Purpose = "import" // leitura do currículo enviado pelo usuário (usa a cadeia principal)
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

// AICall é uma tentativa de chamada a um provider, registrada para as métricas.
type AICall struct {
	Purpose          Purpose
	Provider         string
	Model            string
	Position         int // posição na cadeia (0 = primeiro)
	Status           string
	Duration         time.Duration
	PromptTokens     int
	CompletionTokens int
	JobID            string
	Error            string
}

// CallRecorder guarda as tentativas (implementado sobre o banco). Opcional.
type CallRecorder interface {
	RecordAICall(ctx context.Context, c AICall)
}

type jobIDKey struct{}

type observerKey struct{}

// AttemptObserver acompanha a cadeia ao vivo (ex.: a tela de importação mostra qual modelo
// está tentando e por que os anteriores falharam). Os dois campos são opcionais.
type AttemptObserver struct {
	Start func(p Provider, position int)
	Done  func(c AICall)
}

// WithAttemptObserver faz as chamadas feitas com este contexto avisarem o observador.
func WithAttemptObserver(ctx context.Context, o AttemptObserver) context.Context {
	return context.WithValue(ctx, observerKey{}, o)
}

func observerFrom(ctx context.Context) AttemptObserver {
	o, _ := ctx.Value(observerKey{}).(AttemptObserver)
	return o
}

// WithJobID associa as chamadas feitas com este contexto a um job (para as métricas).
func WithJobID(ctx context.Context, jobID string) context.Context {
	return context.WithValue(ctx, jobIDKey{}, jobID)
}

func jobIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(jobIDKey{}).(string)
	return id
}

// ProviderSource devolve a cadeia configurada no painel. Lista vazia = usar o .env.
type ProviderSource interface {
	Providers(ctx context.Context, purpose Purpose) ([]Provider, error)
}

// resolveProviders monta a cadeia na ordem de prioridade:
// painel (banco) → .env do purpose → (scrape apenas) cadeia principal.
func (ai *AiService) resolveProviders(ctx context.Context, purpose Purpose) []Provider {
	if purpose == PurposeImport {
		// Cadeia principal e, de reserva, a de scrape (sem repetir modelo): a pessoa confere
		// tudo antes de salvar, então um modelo mais fraco é melhor do que nenhum resultado.
		chain := ai.resolveProviders(ctx, PurposeMain)
		seen := map[string]bool{}
		for _, p := range chain {
			seen[p.Name+"|"+p.URL+"|"+p.Model] = true
		}
		for _, p := range ai.resolveProviders(ctx, PurposeScrape) {
			if k := p.Name + "|" + p.URL + "|" + p.Model; !seen[k] {
				seen[k] = true
				chain = append(chain, p)
			}
		}
		return chain
	}
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
func (ai *AiService) withFallback(ctx context.Context, purpose Purpose, call func(ctx context.Context, p Provider, last bool) (Usage, error)) error {
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

		obs := observerFrom(ctx)
		if obs.Start != nil {
			obs.Start(p, i)
		}
		start := time.Now()
		usage, err := call(ctx, p, i == len(providers)-1)
		c := AICall{
			Purpose: purpose, Provider: p.Name, Model: p.Model, Position: i,
			Status: ClassifyError(err), Duration: time.Since(start),
			PromptTokens: usage.PromptTokens, CompletionTokens: usage.CompletionTokens,
			JobID: jobIDFrom(ctx), Error: errorText(err),
		}
		// Cancelada por quem pediu (ex.: botão Cancelar da importação) não é falha do provider:
		// fora das métricas para não derrubar a taxa de sucesso do modelo.
		if !errors.Is(ctx.Err(), context.Canceled) {
			ai.record(ctx, c)
		}
		if obs.Done != nil {
			obs.Done(c)
		}
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

func (ai *AiService) record(ctx context.Context, c AICall) {
	if ai.Recorder == nil {
		return
	}
	// Mesmo com o job cancelado a tentativa precisa ficar registrada.
	ai.Recorder.RecordAICall(context.WithoutCancel(ctx), c)
}

// maxRecordedError é quanto do erro vai para as métricas: o suficiente para depurar
// (ex.: o JSON inteiro de um 429 do Gemini) sem deixar a tabela crescer sem limite.
const maxRecordedError = 4000

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return truncateUTF8(err.Error(), maxRecordedError)
}
