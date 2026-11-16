package discovery

import (
	"reflect"
	"testing"
)

func TestClassifyRoles(t *testing.T) {
	for title, want := range map[string][]string{
		"Desenvolvedor Back-end Pleno":               {"backend"},
		"Backend Engineer (Golang)":                  {"backend"},
		"Engenheiro(a) de Software Full Stack":       {"fullstack"},
		"Desenvolvedor Fullstack (Back-end e Front)": {"fullstack"},
		"Front-end Developer React":                  {"frontend"},
		"Desenvolvedor Go":                           {},
		"Software Engineer":                          {},
		"Engenheiro de Dados Sênior":                 {"data"},
		"Analista de BI":                             {"data"},
		"SRE / DevOps Engineer":                      {"devops"},
		"QA Analyst":                                 {"qa"},
		"Product Designer":                           {"design"},
		"Analista de Suporte N2":                     {"support"},
		"Desenvolvedor Mobile Flutter":               {"mobile"},
		"Backend Engineer - Data Platform":           {"backend", "data"},
		// "API" não é marcador: não vira back-end.
		"Analista de API Management": {},
	} {
		if got := ClassifyRoles(title); !reflect.DeepEqual(got, want) {
			t.Errorf("%q = %v, quer %v", title, got, want)
		}
	}
}

func TestRoleFit(t *testing.T) {
	back := []string{"backend"}
	for _, c := range []struct {
		job  []string
		want string
	}{
		{[]string{"backend"}, RoleMatch},
		{[]string{"backend", "data"}, RoleMatch},
		{[]string{"fullstack"}, RoleNeighbor},
		{[]string{}, RoleUnknown},
		{[]string{"frontend"}, RoleOther},
		{[]string{"data"}, RoleOther},
	} {
		if got := RoleFit(c.job, back); got != c.want {
			t.Errorf("vaga %v para back-end = %s, quer %s", c.job, got, c.want)
		}
	}
	if got := RoleFit([]string{"frontend"}, nil); got != RoleUnknown {
		t.Errorf("sem função procurada = %s", got)
	}
}

func TestParseTarget(t *testing.T) {
	p := ParseTarget("golang backend remoto pleno")
	want := Target{Roles: []string{"backend"}, Technologies: []string{"Go"}, Seniorities: []string{"mid"}, WorkplaceTypes: []string{"remote"}}
	if !reflect.DeepEqual(p.Target, want) || len(p.Unknown) != 0 {
		t.Errorf("golang backend remoto pleno = %+v (não entendeu %v)", p.Target, p.Unknown)
	}

	// "go" sozinho é Go aqui; palavras de ligação não são "não entendi".
	p = ParseTarget("Vaga de desenvolvedor Back-End em Go, híbrido ou remoto, júnior ou pleno")
	want = Target{Roles: []string{"backend"}, Technologies: []string{"Go"}, Seniorities: []string{"junior", "mid"}, WorkplaceTypes: []string{"remote", "hybrid"}}
	if !reflect.DeepEqual(p.Target, want) || len(p.Unknown) != 0 {
		t.Errorf("frase = %+v (não entendeu %v)", p.Target, p.Unknown)
	}

	// Termo mais longo primeiro: "react native" é mobile, não React + "native".
	p = ParseTarget("react native sênior")
	if !reflect.DeepEqual(p.Roles, []string{"mobile"}) || len(p.Unknown) != 0 || !reflect.DeepEqual(p.Seniorities, []string{"senior"}) {
		t.Errorf("react native = %+v (não entendeu %v)", p.Target, p.Unknown)
	}

	p = ParseTarget("node.js full stack curitiba")
	if !reflect.DeepEqual(p.Roles, []string{"fullstack"}) || !reflect.DeepEqual(p.Technologies, []string{"Node.js"}) ||
		!reflect.DeepEqual(p.Unknown, []string{"curitiba"}) {
		t.Errorf("node.js full stack curitiba = %+v (não entendeu %v)", p.Target, p.Unknown)
	}

	p = ParseTarget("")
	if p.Roles == nil || p.Technologies == nil || p.Unknown == nil {
		t.Errorf("texto vazio devolveu lista nula: %+v", p)
	}
}
