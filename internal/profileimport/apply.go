package profileimport

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"hackton-treino/internal/db"
	"hackton-treino/internal/util"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// ApplyRequest é o rascunho depois da conferência: só o que a pessoa marcou para salvar.
// Profile/Links nil = não mexer; campo nil dentro deles = manter o valor atual.
type ApplyRequest struct {
	ImportID     string        `json:"import_id"`
	Profile      *ApplyProfile `json:"profile"`
	Links        *ApplyLinks   `json:"links"`
	Experiences  []Experience  `json:"experiences"`
	Academic     []Academic    `json:"academic"`
	Skills       []Skill       `json:"skills"`
	Projects     []Project     `json:"projects"`
	Certificates []Certificate `json:"certificates"`
}

type ApplyProfile struct {
	FullName     *string `json:"full_name"`
	Phone        *string `json:"phone"`
	ContactEmail *string `json:"contact_email"`
	About        *string `json:"about"`
}

type ApplyLinks struct {
	LinkedinURL  *string `json:"linkedin_url"`
	GithubURL    *string `json:"github_url"`
	PortfolioURL *string `json:"portfolio_url"`
}

type Summary struct {
	Profile       bool `json:"profile"`
	Links         bool `json:"links"`
	Experiences   int  `json:"experiences"`
	Academic      int  `json:"academic"`
	SkillsAdded   int  `json:"skills_added"`
	SkillsUpdated int  `json:"skills_updated"`
	Projects      int  `json:"projects"`
	Certificates  int  `json:"certificates"`
}

// ValidationError é um problema no que a pessoa mandou salvar (vira 400 com a mensagem).
type ValidationError struct{ Msg string }

func (e *ValidationError) Error() string { return e.Msg }

func invalid(format string, args ...any) error {
	return &ValidationError{Msg: fmt.Sprintf(format, args...)}
}

// Validate confere tudo antes de abrir a transação, para a mensagem apontar o item.
func (r *ApplyRequest) Validate() error {
	if len(r.Experiences)+len(r.Academic)+len(r.Projects)+len(r.Certificates) > 4*maxItems || len(r.Skills) > maxSkills {
		return invalid("itens demais para importar de uma vez")
	}
	for _, e := range r.Experiences {
		name := e.JobRole + " em " + e.CompanyName
		switch {
		case strings.TrimSpace(e.CompanyName) == "" || strings.TrimSpace(e.JobRole) == "":
			return invalid("toda experiência precisa de empresa e cargo")
		case NormMonth(e.StartDate) == "":
			return invalid("experiência %q: informe o mês de início", name)
		case e.EndDate != "" && NormMonth(e.EndDate) == "":
			return invalid("experiência %q: data de fim inválida", name)
		case !e.IsCurrentJob && NormMonth(e.EndDate) != "" && NormMonth(e.EndDate) < NormMonth(e.StartDate):
			return invalid("experiência %q: o fim é antes do início", name)
		}
	}
	for _, a := range r.Academic {
		if strings.TrimSpace(a.InstitutionName) == "" || strings.TrimSpace(a.CourseName) == "" {
			return invalid("toda formação precisa de instituição e curso")
		}
		if NormMonth(a.StartDate) == "" {
			return invalid("formação %q: informe o mês de início", a.CourseName)
		}
	}
	for _, s := range r.Skills {
		if SkillKey(s.SkillName) == "" {
			return invalid("habilidade sem nome")
		}
		if !validLevels[s.ProficiencyLevel] {
			return invalid("habilidade %q: escolha o nível", s.SkillName)
		}
	}
	for _, p := range r.Projects {
		if strings.TrimSpace(p.ProjectName) == "" {
			return invalid("todo projeto precisa de nome")
		}
		if NormMonth(p.StartDate) == "" {
			return invalid("projeto %q: informe o mês de início", p.ProjectName)
		}
	}
	for _, c := range r.Certificates {
		// Emissor é opcional: o PDF do LinkedIn costuma listar só o nome.
		if strings.TrimSpace(c.CertificateName) == "" {
			return invalid("todo certificado precisa de nome")
		}
	}
	return nil
}

// Apply grava tudo dentro da transação de q. Habilidades que já existem no perfil
// (mesmo nome) têm só o nível atualizado, nunca são duplicadas.
func Apply(ctx context.Context, q *db.Queries, userID pgtype.UUID, r ApplyRequest, aliases map[string]string) (Summary, error) {
	var sum Summary
	if err := r.Validate(); err != nil {
		return sum, err
	}

	if r.Profile != nil || r.Links != nil {
		cur, err := q.QuerySelectProfile(ctx, userID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return sum, err
		}
		if p := r.Profile; p != nil {
			name := pick(p.FullName, cur.FullName.String, maxShortField)
			if name == "" {
				return sum, invalid("informe o nome completo")
			}
			_, err := q.QueryUpsertProfile(ctx, db.QueryUpsertProfileParams{
				UserID:       userID,
				FullName:     name,
				Phone:        util.ConvertToPgText(pick(p.Phone, cur.Phone.String, 40)),
				About:        util.ConvertToPgText(pick(p.About, cur.About.String, maxLongField)),
				ContactEmail: util.ConvertToPgText(strings.ToLower(pick(p.ContactEmail, cur.ContactEmail.String, maxShortField))),
			})
			if err != nil {
				return sum, err
			}
			sum.Profile = true
		}
		if l := r.Links; l != nil {
			other := cur.OtherLinks
			if len(other) == 0 {
				other = []byte("null")
			}
			_, err := q.QueryUpsertLinks(ctx, db.QueryUpsertLinksParams{
				UserID:       userID,
				LinkedinUrl:  util.ConvertToPgText(pickURL(l.LinkedinURL, cur.LinkedinUrl.String)),
				GithubUrl:    util.ConvertToPgText(pickURL(l.GithubURL, cur.GithubUrl.String)),
				PortfolioUrl: util.ConvertToPgText(pickURL(l.PortfolioURL, cur.PortfolioUrl.String)),
				OtherLinks:   other,
			})
			if err != nil {
				return sum, err
			}
			sum.Links = true
		}
	}

	for _, e := range r.Experiences {
		end := NormMonth(e.EndDate)
		if e.IsCurrentJob {
			end = ""
		}
		_, err := q.QueryInsertExperience(ctx, db.QueryInsertExperienceParams{
			UserID:       userID,
			CompanyName:  clip(e.CompanyName, maxShortField),
			JobRole:      clip(e.JobRole, maxShortField),
			Description:  util.ConvertToPgText(clip(e.Description, maxLongField)),
			IsCurrentJob: e.IsCurrentJob,
			StartDate:    util.ParsePgDate(NormMonth(e.StartDate)),
			EndDate:      util.ParsePgDate(end),
			TechStack:    cleanList(e.TechStack, maxShortField, 30),
			Achievements: cleanList(e.Achievements, 600, maxListEntries),
		})
		if err != nil {
			return sum, err
		}
		sum.Experiences++
	}

	for _, a := range r.Academic {
		_, err := q.QueryInsertAcademicHistory(ctx, db.QueryInsertAcademicHistoryParams{
			UserID:          userID,
			InstitutionName: clip(a.InstitutionName, maxShortField),
			CourseName:      clip(a.CourseName, maxShortField),
			StartDate:       util.ParsePgDate(NormMonth(a.StartDate)),
			EndDate:         util.ParsePgDate(NormMonth(a.EndDate)),
			Description:     util.ConvertToPgText(clip(a.Description, maxLongField)),
		})
		if err != nil {
			return sum, err
		}
		sum.Academic++
	}

	if len(r.Skills) > 0 {
		existing, err := q.QuerySelectAllSkills(ctx, userID)
		if err != nil {
			return sum, err
		}
		byKey := make(map[string]db.UserSkill, len(existing))
		for _, s := range existing {
			byKey[CanonKey(s.SkillName, aliases)] = s
		}
		done := map[string]bool{}
		for _, s := range r.Skills {
			key := CanonKey(s.SkillName, aliases)
			if done[key] {
				continue
			}
			done[key] = true
			if cur, ok := byKey[key]; ok {
				if string(cur.ProficiencyLevel) == s.ProficiencyLevel {
					continue
				}
				err = q.QueryUpdateSkill(ctx, db.QueryUpdateSkillParams{
					ID: cur.ID, UserID: userID, SkillName: cur.SkillName,
					ProficiencyLevel: db.SkillLevel(s.ProficiencyLevel), Tags: cur.Tags,
				})
				sum.SkillsUpdated++
			} else {
				_, err = q.QueryInsertSkill(ctx, db.QueryInsertSkillParams{
					UserID: userID, SkillName: clip(s.SkillName, 80),
					ProficiencyLevel: db.SkillLevel(s.ProficiencyLevel),
				})
				sum.SkillsAdded++
			}
			if err != nil {
				return sum, err
			}
		}
	}

	for _, p := range r.Projects {
		_, err := q.QueryInsertProject(ctx, db.QueryInsertProjectParams{
			UserID:      userID,
			ProjectName: clip(p.ProjectName, maxShortField),
			Description: clip(p.Description, maxLongField),
			ProjectUrl:  util.ConvertToPgText(normURL(p.ProjectURL)),
			Tags:        cleanList(p.Tags, 60, maxListEntries),
			StartDate:   util.ParsePgDate(NormMonth(p.StartDate)),
			EndDate:     util.ParsePgDate(NormMonth(p.EndDate)),
			IsAcademic:  p.IsAcademic,
		})
		if err != nil {
			return sum, err
		}
		sum.Projects++
	}

	for _, c := range r.Certificates {
		_, err := q.QueryInsertCertificate(ctx, db.QueryInsertCertificateParams{
			UserID:              userID,
			CertificateName:     clip(c.CertificateName, maxShortField),
			IssuingOrganization: clip(c.IssuingOrganization, maxShortField),
			IssueDate:           util.ParsePgDate(NormMonth(c.IssueDate)),
			CredentialUrl:       util.ConvertToPgText(normURL(c.CredentialURL)),
		})
		if err != nil {
			return sum, err
		}
		sum.Certificates++
	}
	return sum, nil
}

func pick(v *string, cur string, max int) string {
	if v == nil {
		return cur
	}
	return clip(*v, max)
}

func pickURL(v *string, cur string) string {
	if v == nil {
		return cur
	}
	return normURL(*v)
}

// Existing é o que o perfil já tem, para marcar no rascunho o que seria duplicado.
type Existing struct {
	Experiences  []db.UserExperience
	Academic     []db.UserAcademicHistory
	Skills       []db.UserSkill
	Projects     []db.UserProject
	Certificates []db.UserCertificate
}

// MarkExisting marca os itens do rascunho que já estão no perfil (a tela de conferência
// os deixa desmarcados) e, nas habilidades, mostra o nível atual ao lado do sugerido.
func (d *Draft) MarkExisting(ex Existing, aliases map[string]string) {
	key := func(parts ...string) string {
		for i, p := range parts {
			parts[i] = strings.ToLower(strings.TrimSpace(p))
		}
		return strings.Join(parts, "|")
	}
	month := func(dt pgtype.Date) string {
		if !dt.Valid {
			return ""
		}
		return dt.Time.Format("2006-01")
	}

	exps := map[string]bool{}
	for _, e := range ex.Experiences {
		exps[key(e.CompanyName, e.JobRole, month(e.StartDate))] = true
	}
	for i, e := range d.Experiences {
		d.Experiences[i].Exists = exps[key(e.CompanyName, e.JobRole, e.StartDate)]
	}

	acs := map[string]bool{}
	for _, a := range ex.Academic {
		acs[key(a.InstitutionName, a.CourseName)] = true
	}
	for i, a := range d.Academic {
		d.Academic[i].Exists = acs[key(a.InstitutionName, a.CourseName)]
	}

	skills := map[string]db.UserSkill{}
	for _, s := range ex.Skills {
		skills[CanonKey(s.SkillName, aliases)] = s
	}
	for i, s := range d.Skills {
		if cur, ok := skills[CanonKey(s.SkillName, aliases)]; ok {
			d.Skills[i].ExistingID = cur.ID.String()
			d.Skills[i].ExistingLevel = string(cur.ProficiencyLevel)
		}
	}

	projs := map[string]bool{}
	for _, p := range ex.Projects {
		projs[key(p.ProjectName)] = true
	}
	for i, p := range d.Projects {
		d.Projects[i].Exists = projs[key(p.ProjectName)]
	}

	certs := map[string]bool{}
	for _, c := range ex.Certificates {
		certs[key(c.CertificateName)] = true
	}
	for i, c := range d.Certificates {
		d.Certificates[i].Exists = certs[key(c.CertificateName)]
	}
}
