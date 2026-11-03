package discovery

import (
	"slices"
	"testing"
)

func TestClassifySeniority(t *testing.T) {
	cases := map[string][]string{
		"Desenvolvedor Back-end Pleno":          {LevelMid},
		"Dev Java Sr.":                          {LevelSenior},
		"Engenheiro de Software Pleno/Sênior":   {LevelMid, LevelSenior},
		"Estágio em Desenvolvimento":            {LevelIntern},
		"Analista de Dados Jr":                  {LevelJunior},
		"Staff Software Engineer":               {LevelExpert},
		"Tech Lead Go":                          {LevelExpert},
		"Analista de Sistemas III":              {LevelSenior},
		"Software Engineer II":                  {LevelMid},
		"Desenvolvedor PL/SQL":                  {},
		"Desenvolvedor PL SQL Pleno":            {LevelMid},
		"Desenvolvedor Back-end":                {},
		"Mid-market Account Executive":          {},
		"Customer Support Specialist":           {},
		"Senior Leadership Coordinator":         {LevelSenior, LevelManager},
		"Desenvolvedora Python Júnior (Remoto)": {LevelJunior},
		"Arquiteto de Soluções Cloud":           {LevelExpert},
		// Numeral romano: I = júnior, IV = sênior.
		"Desenvolvedor(a) de Sistemas I":            {LevelJunior},
		"GROWTH SPECIALIST I":                       {LevelJunior},
		"CYBER SECURITY ANALYST I | APPSEC":         {LevelJunior},
		"Programador de Serviços I - Exclusiva PCD": {LevelJunior},
		"Analista de Dados I/II":                    {LevelJunior, LevelMid},
		"Software Engineer IV (Remote)":             {LevelSenior},
		"I Love Code Engineer":                      {},
		"Site Reliability Engineering (SRE)":        {},
		// Gestão.
		"Engineering Manager":                 {LevelManager},
		"Gerente de Projetos de TI":           {LevelManager},
		"Head of Data":                        {LevelManager},
		"Coordenador SRE e DevOps":            {LevelManager},
		"Associate Director of Sales - LATAM": {LevelManager},
		"VP of Engineering":                   {LevelManager},
		"Senior Product Manager":              {LevelSenior, LevelManager},
		"Desenvolvedor Headless CMS":          {},
	}
	for title, want := range cases {
		if got := ClassifySeniority(title); !slices.Equal(got, want) {
			t.Errorf("%q: %v, esperado %v", title, got, want)
		}
	}
	// Campo de nível da Himalayas e do ProgramaThor.
	if got := ClassifySeniority("Entry-level", "Mid-level"); !slices.Equal(got, []string{LevelJunior, LevelMid}) {
		t.Errorf("himalayas: %v", got)
	}
}

func TestMergeSeniority(t *testing.T) {
	got := MergeSeniority([]string{LevelSenior}, []string{LevelMid, LevelSenior})
	if !slices.Equal(got, []string{LevelMid, LevelSenior}) {
		t.Errorf("%v", got)
	}
}

func TestSeniorityPrefs(t *testing.T) {
	all := SeniorityPrefs{}
	if !all.Matches(nil) || !all.Matches([]string{LevelSenior}) {
		t.Error("sem nível marcado não filtra")
	}
	p := SeniorityPrefs{Levels: []string{LevelMid}}
	if p.Matches(nil) || !p.Matches([]string{LevelMid, LevelSenior}) || p.Matches([]string{LevelSenior}) {
		t.Error("filtro de pleno")
	}
	p.IncludeUnknown = true
	if !p.Matches([]string{}) {
		t.Error("vaga sem nível entra com IncludeUnknown")
	}
}
