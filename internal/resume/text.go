package resume

import (
	"strconv"
	"strings"
)

type labels struct {
	summary, experience, skills, projects, education, certifications, languages, present string
	months                                                                               [12]string
}

var labelsByLang = map[string]labels{
	"pt-br": {
		summary: "RESUMO PROFISSIONAL", experience: "EXPERIÊNCIA PROFISSIONAL", skills: "HABILIDADES TÉCNICAS",
		projects: "PROJETOS", education: "FORMAÇÃO ACADÊMICA", certifications: "CERTIFICAÇÕES", languages: "IDIOMAS", present: "Presente",
		months: [12]string{"Jan", "Fev", "Mar", "Abr", "Mai", "Jun", "Jul", "Ago", "Set", "Out", "Nov", "Dez"},
	},
	"en": {
		summary: "SUMMARY", experience: "EXPERIENCE", skills: "SKILLS",
		projects: "PROJECTS", education: "EDUCATION", certifications: "CERTIFICATIONS", languages: "LANGUAGES", present: "Present",
		months: [12]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"},
	},
}

func labelsFor(lang string) labels {
	if l, ok := labelsByLang[lang]; ok {
		return l
	}
	return labelsByLang["pt-br"]
}

// FormatMonth converte "AAAA-MM" em "Nov/2025" (vazio se inválido).
func FormatMonth(ym, lang string) string {
	parts := strings.SplitN(ym, "-", 2)
	if len(parts) != 2 {
		return ""
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil || m < 1 || m > 12 {
		return ""
	}
	return labelsFor(lang).months[m-1] + "/" + parts[0]
}

func period(start, end string, current bool, lang string) string {
	s := FormatMonth(start, lang)
	e := FormatMonth(end, lang)
	if current {
		e = labelsFor(lang).present
	}
	switch {
	case s != "" && e != "":
		return s + " - " + e
	case s != "":
		return s
	default:
		return e
	}
}

// Text gera a versão em texto puro do currículo (pré-visualização na tela e
// exemplos de feedback). O PDF usa o Doc diretamente.
func Text(d Doc) string {
	l := labelsFor(d.Lang)
	var b strings.Builder
	line := func(s string) { b.WriteString(s + "\n") }

	if d.Header.Name != "" {
		line(d.Header.Name)
	}
	var contact []string
	for _, c := range []string{d.Header.Email, d.Header.Phone, d.Header.Linkedin, d.Header.Github, d.Header.Portfolio} {
		if c != "" {
			contact = append(contact, c)
		}
	}
	if len(contact) > 0 {
		line(strings.Join(contact, " | "))
	}

	section := func(title string) { line(""); line(title) }

	if d.Summary != "" {
		section(l.summary)
		line(d.Summary)
	}
	if len(d.Experiences) > 0 {
		section(l.experience)
		for i, e := range d.Experiences {
			if i > 0 {
				line("")
			}
			line(e.Company + " — " + e.Role)
			if p := period(e.Start, e.End, e.Current, d.Lang); p != "" {
				line(p)
			}
			for _, bl := range e.Bullets {
				line("- " + bl)
			}
		}
	}
	if len(d.Skills) > 0 {
		section(l.skills)
		for _, g := range d.Skills {
			line(g.Category + ": " + strings.Join(g.Items, ", "))
		}
	}
	if len(d.Projects) > 0 {
		section(l.projects)
		for i, p := range d.Projects {
			if i > 0 {
				line("")
			}
			line(p.Name)
			if p.URL != "" {
				line(p.URL)
			}
			line(p.Description)
		}
	}
	if len(d.Education) > 0 {
		section(l.education)
		for _, e := range d.Education {
			line(e.Course + " — " + e.Institution)
			if p := period(e.Start, e.End, false, d.Lang); p != "" {
				line(p)
			}
		}
	}
	if len(d.Certifications) > 0 {
		section(l.certifications)
		for _, c := range d.Certifications {
			s := c.Name
			if c.Issuer != "" {
				s += " — " + c.Issuer
			}
			if dt := FormatMonth(c.Date, d.Lang); dt != "" {
				s += " (" + dt + ")"
			}
			line(s)
		}
	}
	if len(d.Languages) > 0 {
		section(l.languages)
		for _, lg := range d.Languages {
			line(lg.Name + " — " + lg.Level)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}
