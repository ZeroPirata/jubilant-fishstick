package profileimport

import (
	"fmt"
	"hackton-treino/internal/resume"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Draft é o que foi entendido do currículo. Nada aqui está salvo: o usuário confere,
// ajusta e só então manda aplicar (ver Apply).
type Draft struct {
	Source       string        `json:"source"` // "linkedin" | "curriculo"
	Profile      Profile       `json:"profile"`
	Links        Links         `json:"links"`
	Experiences  []Experience  `json:"experiences"`
	Academic     []Academic    `json:"academic"`
	Skills       []Skill       `json:"skills"`
	Projects     []Project     `json:"projects"`
	Certificates []Certificate `json:"certificates"`
	Languages    []Language    `json:"languages"`
}

type Profile struct {
	FullName     string `json:"full_name"`
	Phone        string `json:"phone"`
	ContactEmail string `json:"contact_email"`
	About        string `json:"about"`
}

type Links struct {
	LinkedinURL  string `json:"linkedin_url"`
	GithubURL    string `json:"github_url"`
	PortfolioURL string `json:"portfolio_url"`
}

// Datas sempre no formato "AAAA-MM" (o input type=month do perfil).
type Experience struct {
	CompanyName  string   `json:"company_name"`
	JobRole      string   `json:"job_role"`
	Description  string   `json:"description"`
	IsCurrentJob bool     `json:"is_current_job"`
	StartDate    string   `json:"start_date"`
	EndDate      string   `json:"end_date"`
	TechStack    []string `json:"tech_stack"`
	Achievements []string `json:"achievements"`
	Exists       bool     `json:"exists,omitempty"` // já está no perfil
}

type Academic struct {
	InstitutionName string `json:"institution_name"`
	CourseName      string `json:"course_name"`
	StartDate       string `json:"start_date"`
	EndDate         string `json:"end_date"`
	Description     string `json:"description"`
	Exists          bool   `json:"exists,omitempty"`
}

type Skill struct {
	SkillName        string `json:"skill_name"`
	ProficiencyLevel string `json:"proficiency_level"`
	// Explicit: o nível está escrito no currículo ("Go (Avançado)", "especialista em React").
	// Senão o nível é sugerido pelo tempo de uso nas experiências (Months/UsedIn).
	Explicit bool     `json:"explicit"`
	Months   int      `json:"months"`
	UsedIn   []string `json:"used_in"`
	// Preenchidos quando a habilidade já existe no perfil.
	ExistingID    string `json:"existing_id,omitempty"`
	ExistingLevel string `json:"existing_level,omitempty"`
}

type Project struct {
	ProjectName string   `json:"project_name"`
	Description string   `json:"description"`
	ProjectURL  string   `json:"project_url"`
	StartDate   string   `json:"start_date"`
	EndDate     string   `json:"end_date"`
	IsAcademic  bool     `json:"is_academic"`
	Tags        []string `json:"tags"`
	Exists      bool     `json:"exists,omitempty"`
}

type Certificate struct {
	CertificateName     string `json:"certificate_name"`
	IssuingOrganization string `json:"issuing_organization"`
	IssueDate           string `json:"issue_date"`
	CredentialURL       string `json:"credential_url"`
	Exists              bool   `json:"exists,omitempty"`
}

// Language: idioma pelo código quando conhecido ("en"), ou o nome escrito.
type Language struct {
	Language      string `json:"language"`
	Proficiency   string `json:"proficiency"`
	ExistingLevel string `json:"existing_level,omitempty"` // nível que já está no perfil
}

var validLevels = map[string]bool{"basic": true, "intermediate": true, "advanced": true, "expert": true}

// Limites contra respostas fora do normal da IA (e contra abuso no Apply).
const (
	maxItems       = 40
	maxSkills      = 80
	maxShortField  = 200
	maxLongField   = 4000
	maxListEntries = 15
)

// Normalize limpa a resposta da IA: datas no formato certo, níveis válidos, sem itens
// vazios nem duplicados, URLs com https. now é a data de referência para "atual";
// aliases são os sinônimos de stack_aliases (golang → go), usados para não duplicar.
func (d *Draft) Normalize(now time.Time, aliases map[string]string) {
	if d.Source != "linkedin" {
		d.Source = "curriculo"
	}
	d.Profile = Profile{
		FullName:     clip(d.Profile.FullName, maxShortField),
		Phone:        clip(d.Profile.Phone, 40),
		ContactEmail: strings.ToLower(clip(d.Profile.ContactEmail, maxShortField)),
		About:        clip(d.Profile.About, maxLongField),
	}
	d.Links = Links{
		LinkedinURL:  normURL(d.Links.LinkedinURL),
		GithubURL:    normURL(d.Links.GithubURL),
		PortfolioURL: normURL(d.Links.PortfolioURL),
	}

	exps := make([]Experience, 0, len(d.Experiences))
	for _, e := range d.Experiences {
		e.CompanyName = clip(e.CompanyName, maxShortField)
		e.JobRole = clip(e.JobRole, maxShortField)
		if e.CompanyName == "" || e.JobRole == "" {
			continue
		}
		e.Description = clip(e.Description, maxLongField)
		e.StartDate = NormMonth(e.StartDate)
		e.EndDate = NormMonth(e.EndDate)
		if e.EndDate != "" {
			e.IsCurrentJob = false
		}
		e.TechStack = cleanList(e.TechStack, maxShortField, 30)
		e.Achievements = cleanList(e.Achievements, 600, maxListEntries)
		exps = append(exps, e)
	}
	d.Experiences = capSlice(exps, maxItems)

	acs := make([]Academic, 0, len(d.Academic))
	for _, a := range d.Academic {
		a.InstitutionName = clip(a.InstitutionName, maxShortField)
		a.CourseName = clip(a.CourseName, maxShortField)
		if a.InstitutionName == "" || a.CourseName == "" {
			continue
		}
		a.StartDate = NormMonth(a.StartDate)
		a.EndDate = NormMonth(a.EndDate)
		a.Description = clip(a.Description, maxLongField)
		acs = append(acs, a)
	}
	d.Academic = capSlice(acs, maxItems)

	projs := make([]Project, 0, len(d.Projects))
	for _, p := range d.Projects {
		p.ProjectName = clip(p.ProjectName, maxShortField)
		if p.ProjectName == "" {
			continue
		}
		p.Description = clip(p.Description, maxLongField)
		p.ProjectURL = normURL(p.ProjectURL)
		p.StartDate = NormMonth(p.StartDate)
		p.EndDate = NormMonth(p.EndDate)
		p.Tags = cleanList(p.Tags, 60, maxListEntries)
		projs = append(projs, p)
	}
	d.Projects = capSlice(projs, maxItems)

	certs := make([]Certificate, 0, len(d.Certificates))
	for _, c := range d.Certificates {
		c.CertificateName = clip(c.CertificateName, maxShortField)
		if c.CertificateName == "" {
			continue
		}
		c.IssuingOrganization = clip(c.IssuingOrganization, maxShortField)
		c.IssueDate = NormMonth(c.IssueDate)
		c.CredentialURL = normURL(c.CredentialURL)
		certs = append(certs, c)
	}
	d.Certificates = capSlice(certs, maxItems)

	langs := make([]Language, 0, len(d.Languages))
	seenLang := map[string]bool{}
	for _, l := range d.Languages {
		l.Language = resume.LanguageCode(clip(l.Language, 60))
		if l.Language == "" || seenLang[strings.ToLower(l.Language)] {
			continue
		}
		seenLang[strings.ToLower(l.Language)] = true
		if !resume.ValidLanguageLevel(l.Proficiency) {
			l.Proficiency = "intermediate"
		}
		l.ExistingLevel = ""
		langs = append(langs, l)
	}
	d.Languages = capSlice(langs, maxListEntries)

	// "Go" e "Golang" no mesmo cargo viram um só.
	for i := range d.Experiences {
		seen := map[string]bool{}
		stack := d.Experiences[i].TechStack[:0]
		for _, t := range d.Experiences[i].TechStack {
			if k := CanonKey(t, aliases); !seen[k] {
				seen[k] = true
				stack = append(stack, t)
			}
		}
		d.Experiences[i].TechStack = stack
	}
	d.normalizeSkills(now, aliases)
}

// normalizeSkills junta as habilidades citadas na lista com as tecnologias das
// experiências, sem duplicar, e sugere o nível de cada uma.
func (d *Draft) normalizeSkills(now time.Time, aliases map[string]string) {
	seen := map[string]int{}
	skills := make([]Skill, 0, len(d.Skills))
	add := func(s Skill) {
		s.SkillName = clip(s.SkillName, 80)
		key := CanonKey(s.SkillName, aliases)
		if key == "" || resume.IsKnownLanguage(resume.LanguageCode(s.SkillName)) {
			return // idioma não é habilidade técnica (vai em languages)
		}
		lvl := strings.ToLower(strings.TrimSpace(s.ProficiencyLevel))
		if i, ok := seen[key]; ok {
			// Duplicada: fica o nível explícito, se alguma das ocorrências tiver.
			if !skills[i].Explicit && validLevels[lvl] && s.Explicit {
				skills[i].ProficiencyLevel, skills[i].Explicit = lvl, true
			}
			return
		}
		s.Explicit = s.Explicit && validLevels[lvl]
		if s.Explicit {
			s.ProficiencyLevel = lvl
		} else {
			s.ProficiencyLevel = ""
		}
		s.Months, s.UsedIn, s.ExistingID, s.ExistingLevel = 0, nil, "", ""
		seen[key] = len(skills)
		skills = append(skills, s)
	}
	for _, s := range d.Skills {
		add(s)
	}
	for _, e := range d.Experiences {
		for _, t := range e.TechStack {
			add(Skill{SkillName: t})
		}
	}

	for i := range skills {
		s := &skills[i]
		s.Months, s.UsedIn = d.usage(s.SkillName, now, aliases)
		if !s.Explicit {
			s.ProficiencyLevel = LevelFromMonths(s.Months)
		}
	}
	// Mais usadas primeiro: é a ordem em que a pessoa vai conferir.
	sort.SliceStable(skills, func(a, b int) bool { return skills[a].Months > skills[b].Months })
	d.Skills = capSlice(skills, maxSkills)
}

// usage soma o tempo das experiências que usam a habilidade (stack ou texto), sem contar
// duas vezes períodos sobrepostos (ex.: promoções na mesma empresa).
func (d *Draft) usage(skill string, now time.Time, aliases map[string]string) (int, []string) {
	match := skillMatcher(skill)
	key := CanonKey(skill, aliases)
	type span struct{ from, to int } // meses absolutos (ano*12+mês)
	var (
		spans  []span
		usedIn []string
		seenCo = map[string]bool{}
	)
	for _, e := range d.Experiences {
		hit := false
		for _, t := range e.TechStack {
			if CanonKey(t, aliases) == key {
				hit = true
				break
			}
		}
		if !hit {
			hit = match(e.Description + "\n" + strings.Join(e.Achievements, "\n"))
		}
		if !hit {
			continue
		}
		if co := strings.ToLower(e.CompanyName); !seenCo[co] {
			seenCo[co] = true
			usedIn = append(usedIn, e.CompanyName)
		}
		from, ok := monthIndex(e.StartDate)
		if !ok {
			continue
		}
		to, ok := monthIndex(e.EndDate)
		if !ok {
			to = now.Year()*12 + int(now.Month()) - 1
		}
		if to >= from {
			spans = append(spans, span{from, to})
		}
	}

	sort.Slice(spans, func(a, b int) bool { return spans[a].from < spans[b].from })
	total, curFrom, curTo := 0, -1, -1
	for _, s := range spans {
		if curTo < 0 || s.from > curTo+1 {
			if curTo >= 0 {
				total += curTo - curFrom + 1
			}
			curFrom, curTo = s.from, s.to
		} else if s.to > curTo {
			curTo = s.to
		}
	}
	if curTo >= 0 {
		total += curTo - curFrom + 1
	}
	return total, usedIn
}

// LevelFromMonths é a sugestão quando o currículo não diz o nível. "expert" nunca é
// sugerido: tempo de uso sozinho não faz de ninguém referência na área.
func LevelFromMonths(m int) string {
	switch {
	case m >= 36:
		return "advanced"
	case m >= 12:
		return "intermediate"
	default:
		return "basic"
	}
}

// SkillKey compara habilidades ignorando caixa, espaços e pontuação: "Node.js" = "nodejs".
func SkillKey(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case r == '+' || r == '#': // C++, C# são nomes diferentes de C
			b.WriteRune(r)
		case r == ' ' || r == '.' || r == '-' || r == '_' || r == '/':
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Sinônimos que não estão em stack_aliases (versões do mesmo padrão).
var builtinAliases = map[string]string{"html5": "html", "css3": "css"}

// CanonKey é a chave de comparação de uma habilidade: aplica os sinônimos e entende
// "Go (Golang)" e "Google Cloud Platform (GCP)" como o nome de fora ou o de dentro.
func CanonKey(name string, aliases map[string]string) string {
	low := strings.ToLower(strings.TrimSpace(name))
	cands := []string{low}
	if i := strings.Index(low, " ("); i > 0 && strings.HasSuffix(low, ")") {
		cands = append(cands, strings.TrimSpace(low[:i]), strings.TrimSpace(low[i+2:len(low)-1]))
	}
	for _, c := range cands {
		if to, ok := aliases[c]; ok {
			return SkillKey(to)
		}
		if to, ok := builtinAliases[c]; ok {
			return SkillKey(to)
		}
	}
	if len(cands) > 1 {
		return SkillKey(cands[1])
	}
	return SkillKey(low)
}

// skillMatcher procura a habilidade como palavra inteira no texto. Nomes de até 2 letras
// (Go, R, C) só contam com a caixa exata, senão "go" no meio de uma frase viraria Go.
func skillMatcher(skill string) func(string) bool {
	name := strings.TrimSpace(skill)
	if name == "" {
		return func(string) bool { return false }
	}
	flags := "(?i)"
	if utf8.RuneCountInString(name) <= 2 {
		flags = ""
	}
	re, err := regexp.Compile(flags + `(^|[^\pL\pN+#])` + regexp.QuoteMeta(name) + `($|[^\pL\pN+#])`)
	if err != nil {
		return func(string) bool { return false }
	}
	return re.MatchString
}

var (
	reYearMonth = regexp.MustCompile(`^(\d{4})-(\d{1,2})(?:-\d{1,2})?$`)
	reMonthYear = regexp.MustCompile(`^(\d{1,2})/(\d{4})$`)
	reYear      = regexp.MustCompile(`^(\d{4})$`)
)

// NormMonth aceita "2024-03", "2024-03-15", "03/2024" ou "2024" e devolve "AAAA-MM".
// Só o ano vira janeiro. Qualquer outra coisa ("atual", "present") vira vazio.
func NormMonth(s string) string {
	s = strings.TrimSpace(s)
	var y, m int
	if g := reYearMonth.FindStringSubmatch(s); g != nil {
		y, _ = strconv.Atoi(g[1])
		m, _ = strconv.Atoi(g[2])
	} else if g := reMonthYear.FindStringSubmatch(s); g != nil {
		m, _ = strconv.Atoi(g[1])
		y, _ = strconv.Atoi(g[2])
	} else if g := reYear.FindStringSubmatch(s); g != nil {
		y, _ = strconv.Atoi(g[1])
		m = 1
	} else {
		return ""
	}
	if y < 1950 || y > 2100 || m < 1 || m > 12 {
		return ""
	}
	return fmt.Sprintf("%04d-%02d", y, m)
}

func monthIndex(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	t, err := time.Parse("2006-01", s)
	if err != nil {
		return 0, false
	}
	return t.Year()*12 + int(t.Month()) - 1, true
}

func normURL(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || strings.ContainsAny(s, " \t\n") || !strings.Contains(s, ".") {
		return ""
	}
	low := strings.ToLower(s)
	if strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") {
		return clip(s, 500)
	}
	if strings.Contains(low, "://") || strings.HasPrefix(low, "javascript:") || strings.HasPrefix(low, "mailto:") {
		return ""
	}
	return clip("https://"+s, 500)
}

func clip(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	s = s[:max]
	for !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return strings.TrimSpace(s)
}

func cleanList(in []string, maxLen, maxN int) []string {
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, v := range in {
		v = clip(strings.TrimLeft(strings.TrimSpace(v), "-•●▪* "), maxLen)
		k := strings.ToLower(v)
		if v == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, v)
	}
	return capSlice(out, maxN)
}

func capSlice[T any](s []T, n int) []T {
	if len(s) > n {
		return s[:n]
	}
	return s
}
