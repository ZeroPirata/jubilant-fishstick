package profileimport

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"hackton-treino/internal/repository"
	"hackton-treino/internal/repository/aliases"
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
	Logger  *zap.Logger
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
	go s.run(userID, id, text)
	return id, nil
}

func (s *Service) run(userID, id, text string) {
	ctx, cancel := context.WithTimeout(context.Background(), aiTimeout)
	defer cancel()

	st := State{Status: StatusDone}
	draft, err := s.read(ctx, userID, text)
	if err != nil {
		// O texto do currículo nunca vai para o log, só o erro da IA.
		s.Logger.Warn("importação de currículo falhou", zap.String("user_id", userID), zap.Error(err))
		st = State{Status: StatusError, Error: friendlyError(err)}
	} else {
		st.Draft = draft
	}
	if err := s.Store.Finish(context.Background(), userID, id, st); err != nil {
		s.Logger.Error("importação de currículo: falha ao gravar o resultado", zap.Error(err))
	}
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
		errs [5]*repository.RepositoryError
	)
	ex.Experiences, errs[0] = s.Users.QuerySelectAllExperiences(ctx, userID)
	ex.Academic, errs[1] = s.Users.QuerySelectAllAcademicHistories(ctx, userID)
	ex.Skills, errs[2] = s.Users.QuerySelectAllSkills(ctx, userID)
	ex.Projects, errs[3] = s.Users.QuerySelectAllProjects(ctx, userID)
	ex.Certificates, errs[4] = s.Users.QuerySelectAllCertificates(ctx, userID)
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
