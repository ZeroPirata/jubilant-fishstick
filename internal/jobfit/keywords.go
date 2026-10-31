package jobfit

import (
	"regexp"
	"strings"

	"hackton-treino/internal/db"
	"hackton-treino/internal/discovery"
)

// Relatório de palavras-chave (ATS): os sistemas de triagem procuram no currículo os
// termos da vaga. Sem IA: a lista vem da stack extraída e das tecnologias citadas na
// descrição; cada termo é procurado no texto do currículo e nas skills do perfil.

type KeywordGap struct {
	Name  string `json:"name"`
	Skill string `json:"skill"` // skill do perfil que cobre o termo
}

type KeywordReport struct {
	// HasResume: false quando ainda não há currículo gerado (InResume fica vazio e
	// tudo que o perfil cobre vai para MissingInResume).
	HasResume bool `json:"has_resume"`
	// InResume: a vaga pede e o currículo cita.
	InResume []string `json:"in_resume"`
	// MissingInResume: o perfil tem, mas o currículo não cita. Vale gerar de novo ou
	// conferir se a experiência descreve o uso.
	MissingInResume []KeywordGap `json:"missing_in_resume"`
	// NotInProfile: a vaga pede e o perfil não tem. Nunca vai para o currículo sem
	// estar no perfil (o gerador não inventa competências).
	NotInProfile []string `json:"not_in_profile"`
}

// Keywords monta o relatório. stack é a stack extraída da vaga; description, o texto
// completo (para achar tecnologias que a extração deixou de fora); resumeText, o texto
// do currículo ("" sem currículo).
func Keywords(stack []string, description, resumeText string, skills []db.UserSkill, aliases map[string]string) KeywordReport {
	rep := KeywordReport{
		HasResume:       strings.TrimSpace(resumeText) != "",
		InResume:        []string{},
		MissingInResume: []KeywordGap{},
		NotInProfile:    []string{},
	}
	terms := keywordTerms(stack, description, aliases)
	if len(terms) == 0 {
		return rep
	}

	fit := NewEvaluator(skills, aliases).Evaluate(terms)
	byName := make(map[string]Item, len(fit.Items))
	for _, it := range fit.Items {
		byName[it.Name] = it
	}
	resumeLower := strings.ToLower(resumeText)
	for _, term := range terms {
		it := byName[term]
		switch {
		case rep.HasResume && inText(resumeText, resumeLower, term, aliases):
			rep.InResume = append(rep.InResume, term)
		case it.Status == StatusStrong || it.Status == StatusPartial:
			rep.MissingInResume = append(rep.MissingInResume, KeywordGap{Name: term, Skill: it.Skill})
		default:
			rep.NotInProfile = append(rep.NotInProfile, term)
		}
	}
	return rep
}

// keywordTerms junta a stack com as tecnologias do catálogo citadas na descrição, sem
// repetir ("Golang" na descrição não entra se a stack já tem "Go"), com o nome do
// catálogo quando o termo é uma tecnologia conhecida.
func keywordTerms(stack []string, description string, aliases map[string]string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(name string) {
		name = strings.TrimSpace(name)
		key := canonical(name, aliases)
		if key == "" || seen[key] {
			return
		}
		if t, ok := techFor(key); ok {
			if seen["tech:"+t.Name] {
				return
			}
			seen["tech:"+t.Name] = true
			name = t.Name // a stack vem em minúsculas: "postgresql" → "PostgreSQL"
		}
		seen[key] = true
		out = append(out, name)
	}
	for _, s := range stack {
		add(s)
	}
	if strings.TrimSpace(description) != "" {
		for _, n := range discovery.MatchTechs(description, discovery.TechNames) {
			add(n)
		}
	}
	return out
}

// techByTerm: termo em minúsculas (nome ou variação) → tecnologia do catálogo.
var techByTerm = func() map[string]discovery.Technology {
	m := map[string]discovery.Technology{}
	for _, t := range discovery.Technologies {
		m[strings.ToLower(t.Name)] = t
		for _, term := range t.Terms {
			if _, dup := m[term]; !dup {
				m[term] = t
			}
		}
	}
	return m
}()

func techFor(key string) (discovery.Technology, bool) {
	t, ok := techByTerm[key]
	return t, ok
}

// inText diz se o currículo cita o termo. Tecnologia do catálogo usa as variações e
// as regras de contexto do Descobrir ("Golang" conta como Go; "go-live" não); o resto
// é procurado como palavra inteira, também pelos aliases ("k8s" para Kubernetes).
func inText(text, lower, term string, aliases map[string]string) bool {
	key := canonical(term, aliases)
	if t, ok := techFor(key); ok {
		return len(discovery.MatchTechs(text, []string{t.Name})) > 0
	}
	variants := []string{strings.ToLower(term), key}
	for alias, c := range aliases {
		if c == key {
			variants = append(variants, alias)
		}
	}
	for _, v := range variants {
		if v != "" && wordIn(lower, v) {
			return true
		}
	}
	return false
}

// wordIn procura o termo (minúsculo) como palavra inteira: "Java" não casa dentro de
// "JavaScript"; "C#" e "C++" funcionam.
func wordIn(lower, term string) bool {
	return regexp.MustCompile(`(^|[^\pL\pN+#])` + regexp.QuoteMeta(term) + `($|[^\pL\pN+#])`).MatchString(lower)
}
