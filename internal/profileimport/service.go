package profileimport

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"

	"hackton-treino/internal/repository"
	"hackton-treino/internal/repository/aliases"
	"hackton-treino/internal/repository/importlog"
	"hackton-treino/internal/repository/users"
	"hackton-treino/internal/services"

	"go.uber.org/zap"
)

// AI é o pedaço do AiService usado aqui (interface para os testes).
type AI interface {
	ExtractJSON(ctx context.Context, purpose services.Purpose, perProvider time.Duration, systemPrompt, userPrompt string, decode func(content string) error) error
}

type Service struct {
	AI      AI
	Store   *Store
	Users   users.Repository
	Aliases aliases.Repository
	Log     *importlog.Repository // histórico para o painel admin (opcional)
	Logger  *zap.Logger

	mu      sync.Mutex
	running map[string]context.CancelFunc // leituras deste processo, para o Cancelar
}

// LoadAliases devolve os sinônimos de stack (golang → go). Sem eles a importação
// funciona, só junta menos variações de nome.
func (s *Service) LoadAliases(ctx context.Context) map[string]string {
	if s.Aliases == nil {
		return nil
	}
	m, err := s.Aliases.QuerySelectAllStackAliases(ctx)
	if err != nil {
		s.Logger.Warn("importação de currículo: falha ao carregar stack_aliases", zap.String("erro", err.Message))
		return nil
	}
	return m
}

// aiTimeout cobre a cadeia inteira; cada modelo tem no máximo providerTimeout, para um
// modelo lento (NVIDIA no plano gratuito oscila de 1,5 a 5+ min) não esgotar a vez dos outros.
const (
	aiTimeout       = 7 * time.Minute
	providerTimeout = 150 * time.Second
)

// Start lê o arquivo na hora (erro de formato volta direto para a pessoa) e deixa a IA
// rodando em segundo plano. Devolve o id para consultar o andamento.
func (s *Service) Start(ctx context.Context, userID string, data []byte) (string, error) {
	text, err := ExtractText(ctx, data)
	if err != nil {
		return "", err
	}
	id, err := newID()
	if err != nil {
		return "", err
	}
	if err := s.Store.Begin(ctx, userID, id); err != nil {
		return "", err
	}
	s.history("início", func(ctx context.Context, l *importlog.Repository) error {
		return l.Start(ctx, id, userID, Kind(data), len(data))
	})
	ctx, cancel := context.WithTimeout(context.Background(), aiTimeout)
	s.mu.Lock()
	if s.running == nil {
		s.running = map[string]context.CancelFunc{}
	}
	s.running[id] = cancel
	s.mu.Unlock()
	go s.run(ctx, cancel, userID, id, text)
	return id, nil
}

// Cancel interrompe a chamada de IA em andamento (se for deste processo) e apaga o estado.
func (s *Service) Cancel(ctx context.Context, userID, id string) {
	s.mu.Lock()
	cancel := s.running[id]
	s.mu.Unlock()
	s.Store.Cancel(ctx, userID, id)
	if cancel != nil {
		cancel()
	}
	s.history("cancelamento", func(ctx context.Context, l *importlog.Repository) error { return l.Cancel(ctx, id, userID) })
}

// Applied marca no histórico que a pessoa salvou o rascunho no perfil.
func (s *Service) Applied(userID, id string) {
	s.history("aplicação", func(ctx context.Context, l *importlog.Repository) error { return l.Applied(ctx, id, userID) })
}

// history grava no histórico do admin sem nunca atrapalhar a importação.
func (s *Service) history(what string, fn func(context.Context, *importlog.Repository) error) {
	if s.Log == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := fn(ctx, s.Log); err != nil {
		s.Logger.Warn("importação de currículo: falha ao gravar o histórico", zap.String("etapa", what), zap.Error(err))
	}
}

func (s *Service) run(ctx context.Context, cancel context.CancelFunc, userID, id, text string) {
	defer func() {
		cancel()
		s.mu.Lock()
		delete(s.running, id)
		s.mu.Unlock()
	}()

	// Andamento ao vivo: qual modelo está tentando e como foram os anteriores.
	st := State{Status: StatusProcessing, StartedAt: time.Now()}
	var (
		mu     sync.Mutex
		logged []importlog.Attempt // com provider e erro completo: só para o histórico do admin
	)
	save := func() {
		mu.Lock()
		snap := st
		snap.Attempts = append([]Attempt(nil), st.Attempts...)
		mu.Unlock()
		if err := s.Store.Update(context.Background(), userID, id, snap); err != nil {
			s.Logger.Warn("importação de currículo: falha ao gravar o andamento", zap.Error(err))
		}
	}
	ctx = services.WithAttemptObserver(ctx, services.AttemptObserver{
		Start: func(p services.Provider, _ int) {
			mu.Lock()
			st.Current, st.CurrentAt = modelName(p.Model, p.Name), time.Now()
			mu.Unlock()
			save()
		},
		Done: func(c services.AICall) {
			mu.Lock()
			st.Current, st.CurrentAt = "", time.Time{}
			st.Attempts = append(st.Attempts, Attempt{Model: modelName(c.Model, c.Provider), Status: c.Status, Seconds: c.Duration.Seconds()})
			status := c.Status
			if errors.Is(ctx.Err(), context.Canceled) {
				status = "canceled" // a pessoa cancelou: não foi falha do modelo
			}
			logged = append(logged, importlog.Attempt{Model: modelName(c.Model, c.Provider), Provider: c.Provider,
				Status: status, Seconds: c.Duration.Seconds(), Error: c.Error})
			snap := append([]importlog.Attempt(nil), logged...)
			mu.Unlock()
			save()
			s.history("tentativa", func(ctx context.Context, l *importlog.Repository) error { return l.SetAttempts(ctx, id, snap) })
		},
	})

	draft, err := s.read(ctx, userID, text)
	mu.Lock()
	final := State{Status: StatusDone, StartedAt: st.StartedAt, Attempts: st.Attempts, Draft: draft}
	mu.Unlock()
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return // cancelada pela pessoa: o estado já foi apagado
		}
		// O texto do currículo nunca vai para o log, só o erro da IA.
		s.Logger.Warn("importação de currículo falhou", zap.String("user_id", userID), zap.Error(err))
		final.Status, final.Error, final.Draft = StatusError, friendlyError(err), nil
	}
	if err := s.Store.Finish(context.Background(), userID, id, final); err != nil {
		s.Logger.Error("importação de currículo: falha ao gravar o resultado", zap.Error(err))
	}
	var counts *importlog.Counts
	adminErr := ""
	if err != nil {
		adminErr = clip(err.Error(), 2000) // erro técnico completo (a pessoa vê o amigável)
	}
	if draft != nil && err == nil {
		counts = &importlog.Counts{Experiences: len(draft.Experiences), Academic: len(draft.Academic),
			Skills: len(draft.Skills), Projects: len(draft.Projects), Certificates: len(draft.Certificates),
			Languages: len(draft.Languages)}
	}
	s.history("resultado", func(ctx context.Context, l *importlog.Repository) error {
		return l.Finish(ctx, id, final.Status, adminErr, counts)
	})
}

// modelName mostra só o nome do modelo ("google/gemma-4-31b-it" → "gemma-4-31b-it").
func modelName(model, provider string) string {
	if model == "" {
		return provider
	}
	if i := strings.LastIndex(model, "/"); i >= 0 {
		return model[i+1:]
	}
	return model
}

func (s *Service) read(ctx context.Context, userID, text string) (*Draft, error) {
	var draft *Draft
	err := s.AI.ExtractJSON(ctx, services.PurposeImport, providerTimeout, SystemPrompt, UserPrompt(text), func(content string) error {
		d, err := Decode(content)
		draft = d
		return err
	})
	if err != nil {
		return nil, err
	}
	aliasMap := s.LoadAliases(ctx)
	draft.Normalize(time.Now(), aliasMap)

	// O que já está no perfil: sem isso, confirmar tudo duplicaria as experiências.
	var (
		ex   Existing
		errs [6]*repository.RepositoryError
	)
	ex.Experiences, errs[0] = s.Users.QuerySelectAllExperiences(ctx, userID)
	ex.Academic, errs[1] = s.Users.QuerySelectAllAcademicHistories(ctx, userID)
	ex.Skills, errs[2] = s.Users.QuerySelectAllSkills(ctx, userID)
	ex.Projects, errs[3] = s.Users.QuerySelectAllProjects(ctx, userID)
	ex.Certificates, errs[4] = s.Users.QuerySelectAllCertificates(ctx, userID)
	ex.Languages, errs[5] = s.Users.QuerySelectAllLanguages(ctx, userID)
	for _, e := range errs {
		if e != nil {
			return nil, errors.New("carregar o perfil atual: " + e.Message)
		}
	}
	draft.MarkExisting(ex, aliasMap)
	return draft, nil
}

func friendlyError(err error) string {
	var rl *services.ErrRateLimit
	switch {
	case errors.As(err, &rl):
		return "Os modelos de IA estão no limite de uso agora. Tente de novo em alguns minutos."
	case errors.Is(err, context.DeadlineExceeded):
		return "A leitura demorou demais. Tente de novo em alguns minutos."
	default:
		return "Não consegui entender o currículo agora. Tente de novo; se continuar, preencha o perfil manualmente."
	}
}

func newID() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
