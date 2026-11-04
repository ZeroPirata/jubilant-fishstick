package handler

import (
	"hackton-treino/internal/db"
	"hackton-treino/internal/discovery"
	"hackton-treino/internal/jobfit"
	"hackton-treino/internal/middleware"
	"hackton-treino/internal/util"
	"net/http"
)

// DiscoveredJobDetail: a página "Sobre a vaga" de uma vaga do catálogo, aberta pelo
// sino ou pelo "Ver mais" do Descobrir, para o usuário decidir se gera o currículo.
type DiscoveredJobDetail struct {
	DiscoveredJob
	Description string `json:"description"`
}

// GetJob (GET /discovery/jobs/{id}): a vaga com a descrição inteira, a compatibilidade
// (a mesma conta da lista: leitura da IA quando já está na pipeline), a vaga na
// pipeline, se foi dispensada e os outros sites onde ela aparece.
func (h *DiscoveryHandler) GetJob(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, ErrNotAuthorized.Error())
		return
	}
	jobID, errID := util.ParseUUID(r.PathValue("id"))
	if errID != nil {
		writeError(w, http.StatusBadRequest, "id inválido")
		return
	}
	ctx := r.Context()
	row, errR := h.Repo.JobGet(ctx, db.DiscoveryJobGetParams{ID: jobID, UserID: userID})
	if errR != nil {
		writeRepositoryError(w, errR)
		return
	}
	skills, errR := h.Users.QuerySelectAllSkills(ctx, userID.String())
	if errR != nil {
		writeRepositoryError(w, errR)
		return
	}
	aliasMap, errA := h.Aliases.QuerySelectAllStackAliases(ctx)
	if errA != nil {
		aliasMap = map[string]string{}
	}
	pipeStacks, errR := h.Repo.PipelineStacks(ctx, userID)
	if errR != nil {
		writeRepositoryError(w, errR)
		return
	}
	dismissed, errR := h.Repo.DismissedIDs(ctx, userID)
	if errR != nil {
		writeRepositoryError(w, errR)
		return
	}
	prefs, errR := h.Repo.SelectPreferences(ctx, userID)
	if errR != nil {
		writeRepositoryError(w, errR)
		return
	}

	matchSkills, _ := strongSkills(skills)
	job := toDiscoveredJob(db.DiscoveryListCandidatesRow{
		ID: row.ID, Source: row.Source, Title: row.Title, Company: row.Company,
		Location: row.Location, WorkplaceType: row.WorkplaceType, International: row.International,
		Url: row.Url, Seniority: row.Seniority, PublishedAt: row.PublishedAt, Techs: row.Techs,
		Language: row.Language, FirstSeenAt: row.FirstSeenAt,
	}, discovery.PlanSkills(matchSkills).Matched(row.Techs, nil))
	job.MatchedTechs = intersect(prefs.Technologies, row.Techs)
	job.Dismissed = dismissed[row.ID.Bytes]
	job.Snippet = snippet(row.Description, 280)
	if job.Language == "" {
		job.Language = discovery.GuessLanguage(row.Title + "\n" + row.Description)
	}
	if row.PipelineJobID.Valid {
		job.AlreadyAdded = true
		job.PipelineJobID = row.PipelineJobID.String()
		job.PipelineStatus = row.PipelineStatus
	}
	var fit jobfit.Fit
	fit, job.FitFrom = jobFit(jobfit.NewEvaluator(skills, aliasMap), row.Techs, pipeStacks[row.Url])
	job.Fit = &fit
	job.Stack = otherStack(job)

	if row.Company != "" {
		sibs, errR := h.Repo.JobSiblings(ctx, db.DiscoveryJobSiblingsParams{ID: row.ID, Title: row.Title, Company: row.Company})
		if errR != nil {
			writeRepositoryError(w, errR)
			return
		}
		for _, s := range sibs {
			job.addAlsoOn(s.Source, s.Url)
		}
	}
	writeJSON(w, http.StatusOK, DiscoveredJobDetail{DiscoveredJob: job, Description: row.Description})
}
