package worker

import (
	"context"
	"hackton-treino/internal/db"
	"hackton-treino/internal/resume"
	"hackton-treino/internal/services"
	"regexp"
	"strings"
	"time"

	"go.uber.org/zap"
)

// O prompt pede {{CANDIDATO_NOME}}, mas o modelo às vezes escreve [CANDIDATO_NOME],
// {CANDIDATO_NOME}, [Seu nome], [Your Name]...: tudo vira o nome do perfil.
var reNamePlaceholder = regexp.MustCompile(`(?i)\{\{\s*candidato_nome\s*\}\}|[\[{<]\s*(?:candidato_nome|nome do candidato|seu nome|nome completo|your name|candidate name|full name)\s*[\]}>]`)

// Placeholder que sobrou sem valor: {{ALGO}} ou [ALGO_EM_MAIUSCULAS].
var rePlaceholder = regexp.MustCompile(`\{\{[^}]+\}\}|\[[A-Z][A-Z0-9_ ]{2,}\]`)

func fillCoverLetter(text, name string) string {
	text = reNamePlaceholder.ReplaceAllLiteralString(text, name)
	text = rePlaceholder.ReplaceAllString(text, "")
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// storedResume é o formato de generated_resumes.content_json a partir da versão 2.
// Curriculo (texto) continua sendo gravado para a pré-visualização da tela e para
// os exemplos de feedback; o PDF é gerado a partir de Doc.
type storedResume struct {
	Version     int         `json:"version"`
	Doc         *resume.Doc `json:"doc,omitempty"`
	Curriculo   string      `json:"curriculo"`
	CoverLetter string      `json:"cover_letter"`
}

// buildSource reúne os fatos do banco na mesma ordem (ids) usada em buildUserPrompt.
func (w *Worker) buildSource(job *db.Job, match *services.MatchResult, profile db.QuerySelectProfileRow) resume.Source {
	lang := "pt-br"
	if job.Language.String == "en" {
		lang = "en"
	}

	email := profile.Email
	if profile.ContactEmail.Valid && profile.ContactEmail.String != "" {
		email = profile.ContactEmail.String
	}

	src := resume.Source{
		Lang: lang,
		Header: resume.Header{
			Name:      profile.FullName.String,
			Email:     email,
			Phone:     profile.Phone.String,
			Linkedin:  profile.LinkedinUrl.String,
			Github:    profile.GithubUrl.String,
			Portfolio: profile.PortfolioUrl.String,
		},
		Experiences: toSourceExperiences(selectedExperiences(match)),
		JobStack:    job.TechStack,
		Aliases:     w.aliases,
	}

	for _, p := range selectedProjects(match) {
		src.Projects = append(src.Projects, resume.SourceProject{Name: p.ProjectName, Description: p.Description, URL: p.ProjectUrl.String})
	}
	for _, s := range match.Habilidades {
		src.Skills = append(src.Skills, s.SkillName)
	}
	for _, f := range match.Formacoes {
		e := resume.Education{Institution: f.InstitutionName, Course: f.CourseName}
		if f.StartDate.Valid {
			e.Start = resume.MonthYear(f.StartDate.Time)
		}
		if f.EndDate.Valid {
			e.End = resume.MonthYear(f.EndDate.Time)
		}
		src.Education = append(src.Education, e)
	}
	for _, c := range match.Certificacoes {
		cert := resume.Certification{Name: c.CertificateName, Issuer: c.IssuingOrganization, URL: c.CredentialUrl.String}
		if c.IssueDate.Valid {
			cert.Date = resume.MonthYear(c.IssueDate.Time)
		}
		src.Certifications = append(src.Certifications, cert)
	}
	for _, l := range match.Idiomas {
		src.Languages = append(src.Languages, resume.Language{
			Name: resume.LanguageName(l.Language, lang), Level: resume.LanguageLevelName(l.Proficiency, lang),
		})
	}
	return src
}

// generateWithCorrection chama a IA e, se o resumo quebrar uma regra que o código
// detecta (ex.: "0 anos", formação inexistente), pede UMA correção explícita.
// Se ainda assim vier errado, remove as frases problemáticas.
func (w *Worker) generateWithCorrection(ctx context.Context, jobID, systemPrompt, userPrompt string, src resume.Source) (*resume.LLMOutput, error) {
	out, err := w.LLM.GenerateCurriculum(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	violations := resume.SummaryViolations(out.Resumo, src, time.Now())
	if len(violations) == 0 {
		return out, nil
	}
	w.Logger.Warn("Resumo violou regras, pedindo correção", zap.String("job_id", jobID), zap.Strings("violacoes", violations))

	header := "CORREÇÕES OBRIGATÓRIAS (a resposta anterior errou):"
	if src.Lang == "en" {
		header = "MANDATORY CORRECTIONS (the previous answer was wrong):"
	}
	retry, errRetry := w.LLM.GenerateCurriculum(ctx, systemPrompt, userPrompt+"\n\n"+header+"\n- "+strings.Join(violations, "\n- "))
	if errRetry == nil {
		out = retry
	} else {
		w.Logger.Warn("Correção falhou, usando a primeira resposta", zap.String("job_id", jobID), zap.Error(errRetry))
	}

	if still := resume.SummaryViolations(out.Resumo, src, time.Now()); len(still) > 0 {
		w.Logger.Warn("Resumo ainda viola regras, removendo frases", zap.String("job_id", jobID), zap.Strings("violacoes", still))
		out.Resumo = resume.StripViolatingSentences(out.Resumo, src, time.Now())
	}
	return out, nil
}
