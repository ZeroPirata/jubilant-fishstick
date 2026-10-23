package worker

import (
	"encoding/json"
	"fmt"
	"hackton-treino/internal/db"
	"hackton-treino/internal/resume"
	"hackton-treino/internal/services"
	"strings"
	"time"
)

const (
	maxExperiencias = 5
	maxProjetos     = 6
	maxExemploChars = 1500
)

// Os ids enviados à IA são os índices destas listas; buildSource usa as mesmas,
// então a IA e o código sempre se referem aos mesmos itens.
func selectedExperiences(match *services.MatchResult) []db.UserExperience {
	if len(match.Experiencias) > maxExperiencias {
		return match.Experiencias[:maxExperiencias]
	}
	return match.Experiencias
}

func selectedProjects(match *services.MatchResult) []db.UserProject {
	if len(match.Projetos) > maxProjetos {
		return match.Projetos[:maxProjetos]
	}
	return match.Projetos
}

// exemploCompacto reduz um currículo bem avaliado ao que serve de referência de
// qualidade (resumo e bullets), em vez de mandar o content_json inteiro com a carta.
func exemploCompacto(contentJSON []byte) string {
	var c struct {
		Version   int        `json:"version"`
		Doc       resume.Doc `json:"doc"`
		Curriculo string     `json:"curriculo"`
	}
	if err := json.Unmarshal(contentJSON, &c); err != nil {
		return ""
	}
	if c.Version >= resume.DocVersion {
		parts := []string{c.Doc.Summary}
		for _, e := range c.Doc.Experiences {
			for _, b := range e.Bullets {
				parts = append(parts, "- "+b)
			}
		}
		return truncateRunes(strings.Join(parts, "\n"), maxExemploChars)
	}
	return truncateRunes(c.Curriculo, maxExemploChars)
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

func toSourceExperiences(exps []db.UserExperience) []resume.SourceExperience {
	out := make([]resume.SourceExperience, 0, len(exps))
	for _, e := range exps {
		se := resume.SourceExperience{
			Company: e.CompanyName, Role: e.JobRole, Description: e.Description.String,
			Current: e.IsCurrentJob, Stack: e.TechStack, Achievements: e.Achievements,
		}
		if e.StartDate.Valid {
			se.Start = e.StartDate.Time
		}
		if e.EndDate.Valid {
			se.End = e.EndDate.Time
		}
		out = append(out, se)
	}
	return out
}

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

	experiencias := selectedExperiences(match)

	prompt := userPrompt{
		Vaga:            vagaP,
		AnosExperiencia: resume.YearsOfExperience(toSourceExperiences(experiencias), time.Now()),
		PrimeiroEmprego: len(experiencias) == 0,
		Experiencias:    make([]experienciaPrompt, 0, len(match.Experiencias)),
		Habilidades:     make([]habilidadePrompt, 0, len(match.Habilidades)),
		Projetos:        make([]projetoPrompt, 0, len(match.Projetos)),
		Formacoes:       make([]formacaoPrompt, 0, len(match.Formacoes)),
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
		if ex := exemploCompacto(e); ex != "" {
			prompt.Feedback.ExemplosExcelentes = append(prompt.Feedback.ExemplosExcelentes, ex)
		}
	}

	for i, e := range experiencias {
		conquistas := e.Achievements
		if len(conquistas) > 4 {
			conquistas = conquistas[:4]
		}
		ep := experienciaPrompt{
			ID:         i,
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

	// Até 6 projetos mais recentes — o LLM escolhe os mais relevantes
	for i, p := range selectedProjects(match) {
		prompt.Projetos = append(prompt.Projetos, projetoPrompt{
			ID:        i,
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
