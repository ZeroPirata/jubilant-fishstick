package worker

import (
	"context"
	"errors"
	"hackton-treino/internal/db"
	ucache "hackton-treino/internal/repository/cache"
	"hackton-treino/internal/services"

	"go.uber.org/zap"
)

// matchComBancoPessoal busca todo o banco pessoal do usuário sem filtrar por tags.
//
// Filtragem por tags via `&&` no Postgres é case-sensitive e dependente de o usuário
// ter cadastrado exatamente as mesmas keywords da vaga. Para um portfólio pessoal com
// poucas entidades o custo de enviar tudo ao LLM é insignificante; a LLM faz a seleção
// de relevância melhor do que qualquer heurística de tag matching local.
func (w *Worker) matchComBancoPessoal(ctx context.Context, job *db.Job) (services.MatchResult, error) {
	userID := job.UserID.String()
	jobID := job.ID.String()

	experiencias, err := w.cachedExperiences(ctx, userID, jobID)
	if err != nil {
		return services.MatchResult{}, err
	}

	habilidades, err := w.cachedSkills(ctx, userID, jobID)
	if err != nil {
		return services.MatchResult{}, err
	}

	projetos, err := w.cachedProjects(ctx, userID, jobID)
	if err != nil {
		return services.MatchResult{}, err
	}

	formacoes, err := w.cachedAcademic(ctx, userID, jobID)
	if err != nil {
		return services.MatchResult{}, err
	}

	certificacoes, err := w.cachedCertificates(ctx, userID, jobID)
	if err != nil {
		return services.MatchResult{}, err
	}

	excelentes, _ := w.Feedbacks.QuerySelectExcellentResumes(ctx, userID)
	feedbacks, _ := w.Feedbacks.QuerySelectGoodFeedbacks(ctx, userID)

	return services.MatchResult{
		Experiencias:  experiencias,
		Habilidades:   habilidades,
		Projetos:      projetos,
		Formacoes:     formacoes,
		Certificacoes: certificacoes,
		Excelentes:    excelentes,
		Feedbacks:     feedbacks,
	}, nil
}

func (w *Worker) cachedExperiences(ctx context.Context, userID, jobID string) ([]db.UserExperience, error) {
	if cached, hit := ucache.GetTyped[[]db.UserExperience](ctx, w.Cache, userID, ucache.TopicExperiences); hit {
		return cached, nil
	}
	rows, errR := w.Users.QuerySelectAllExperiences(ctx, userID)
	if errR != nil {
		w.Logger.Error("Erro ao buscar experiências", zap.String("job_id", jobID), zap.Error(errR))
		return nil, errors.New("erro ao buscar experiências")
	}
	_ = ucache.SetTyped(ctx, w.Cache, userID, ucache.TopicExperiences, rows)
	return rows, nil
}

func (w *Worker) cachedSkills(ctx context.Context, userID, jobID string) ([]db.UserSkill, error) {
	if cached, hit := ucache.GetTyped[[]db.UserSkill](ctx, w.Cache, userID, ucache.TopicSkills); hit {
		return cached, nil
	}
	rows, errR := w.Users.QuerySelectAllSkills(ctx, userID)
	if errR != nil {
		w.Logger.Error("Erro ao buscar habilidades", zap.String("job_id", jobID), zap.Error(errR))
		return nil, errors.New("erro ao buscar habilidades")
	}
	_ = ucache.SetTyped(ctx, w.Cache, userID, ucache.TopicSkills, rows)
	return rows, nil
}

func (w *Worker) cachedProjects(ctx context.Context, userID, jobID string) ([]db.UserProject, error) {
	if cached, hit := ucache.GetTyped[[]db.UserProject](ctx, w.Cache, userID, ucache.TopicProjects); hit {
		return cached, nil
	}
	rows, errR := w.Users.QuerySelectAllProjects(ctx, userID)
	if errR != nil {
		w.Logger.Error("Erro ao buscar projetos", zap.String("job_id", jobID), zap.Error(errR))
		return nil, errors.New("erro ao buscar projetos")
	}
	_ = ucache.SetTyped(ctx, w.Cache, userID, ucache.TopicProjects, rows)
	return rows, nil
}

func (w *Worker) cachedAcademic(ctx context.Context, userID, jobID string) ([]db.UserAcademicHistory, error) {
	if cached, hit := ucache.GetTyped[[]db.UserAcademicHistory](ctx, w.Cache, userID, ucache.TopicAcademic); hit {
		return cached, nil
	}
	rows, errR := w.Users.QuerySelectAllAcademicHistories(ctx, userID)
	if errR != nil {
		w.Logger.Error("Erro ao buscar formações acadêmicas", zap.String("job_id", jobID), zap.Error(errR))
		return nil, errors.New("erro ao buscar formações")
	}
	_ = ucache.SetTyped(ctx, w.Cache, userID, ucache.TopicAcademic, rows)
	return rows, nil
}

func (w *Worker) cachedCertificates(ctx context.Context, userID, jobID string) ([]db.UserCertificate, error) {
	if cached, hit := ucache.GetTyped[[]db.UserCertificate](ctx, w.Cache, userID, ucache.TopicCertificates); hit {
		return cached, nil
	}
	rows, errR := w.Users.QuerySelectAllCertificates(ctx, userID)
	if errR != nil {
		w.Logger.Error("Erro ao buscar certificações", zap.String("job_id", jobID), zap.Error(errR))
		return nil, errors.New("erro ao buscar certificações")
	}
	_ = ucache.SetTyped(ctx, w.Cache, userID, ucache.TopicCertificates, rows)
	return rows, nil
}
