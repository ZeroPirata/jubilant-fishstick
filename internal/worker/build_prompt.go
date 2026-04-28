package worker

import (
	"encoding/json"
	"fmt"
	"hackton-treino/internal/db"
	"hackton-treino/internal/services"
)

func (w *Worker) buildUserPrompt(job *db.Job, match *services.MatchResult) (string, error) {
	vagaP := vagaPrompt{
		Empresa:    job.CompanyName.String,
		Titulo:     job.JobTitle.String,
		Stack:      job.TechStack,
		Requisitos: job.Requirements,
	}
	if len(job.Requirements) == 0 {
		vagaP.Descricao = job.Description.String
	}

	prompt := userPrompt{
		Vaga:         vagaP,
		Experiencias: make([]experienciaPrompt, 0, len(match.Experiencias)),
		Habilidades:  make([]habilidadePrompt, 0, len(match.Habilidades)),
		Projetos:     make([]projetoPrompt, 0, len(match.Projetos)),
		Formacoes:    make([]formacaoPrompt, 0, len(match.Formacoes)),
		Feedback: feedbackPrompt{
			ExemplosExcelentes:    make([]string, 0, len(match.Excelentes)),
			ComentariosAnteriores: match.Feedbacks,
		},
	}

	const maxExcelentes = 1
	excelentes := match.Excelentes
	if len(excelentes) > maxExcelentes {
		excelentes = excelentes[:maxExcelentes]
	}
	for _, e := range excelentes {
		prompt.Feedback.ExemplosExcelentes = append(prompt.Feedback.ExemplosExcelentes, string(e))
	}

	const maxExperiencias = 5
	experiencias := match.Experiencias
	if len(experiencias) > maxExperiencias {
		experiencias = experiencias[:maxExperiencias]
	}

	for _, e := range experiencias {
		conquistas := e.Achievements
		if len(conquistas) > 4 {
			conquistas = conquistas[:4]
		}
		ep := experienciaPrompt{
			Empresa:    e.CompanyName,
			Cargo:      e.JobRole,
			Descricao:  e.Description.String,
			Atual:      e.IsCurrentJob,
			Stack:      e.TechStack,
			Conquistas: conquistas,
		}
		if e.StartDate.Valid {
			ep.DataInicio = fmt.Sprintf("%04d-%02d", e.StartDate.Time.Year(), e.StartDate.Time.Month())
		}
		if e.EndDate.Valid {
			ep.DataFim = fmt.Sprintf("%04d-%02d", e.EndDate.Time.Year(), e.EndDate.Time.Month())
		}
		prompt.Experiencias = append(prompt.Experiencias, ep)
	}

	for _, h := range match.Habilidades {
		prompt.Habilidades = append(prompt.Habilidades, habilidadePrompt{
			Nome:  h.SkillName,
			Nivel: string(h.ProficiencyLevel),
		})
	}

	// Limitar a 6 projetos mais recentes — o LLM escolhe os mais relevantes
	projetos := match.Projetos
	const maxProjetos = 6
	if len(projetos) > maxProjetos {
		projetos = projetos[:maxProjetos]
	}
	for _, p := range projetos {
		prompt.Projetos = append(prompt.Projetos, projetoPrompt{
			Nome:      p.ProjectName,
			Descricao: p.Description,
			Link:      p.ProjectUrl.String,
		})
	}

	for _, f := range match.Formacoes {
		fp := formacaoPrompt{
			Instituicao: f.InstitutionName,
			Curso:       f.CourseName,
		}
		if f.StartDate.Valid {
			fp.DataInicio = fmt.Sprintf("%04d-%02d", f.StartDate.Time.Year(), f.StartDate.Time.Month())
		}
		if f.EndDate.Valid {
			fp.DataFim = fmt.Sprintf("%04d-%02d", f.EndDate.Time.Year(), f.EndDate.Time.Month())
		}
		prompt.Formacoes = append(prompt.Formacoes, fp)
	}

	if job.Mode == "resume_only" || job.Mode == "cover_only" {
		prompt.Modo = job.Mode
	}

	raw, err := json.Marshal(prompt)
	if err != nil {
		return "", fmt.Errorf("build prompt: %w", err)
	}

	return string(raw), nil
}
