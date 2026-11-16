package linkedinsearch

import (
	"hackton-treino/internal/discovery"
	"reflect"
	"strings"
	"testing"
)

func TestFromTargetGolangBackend(t *testing.T) {
	got := FromTarget(discovery.Target{
		Roles: []string{"backend"}, Technologies: []string{"Go"},
		Seniorities: []string{"mid"}, WorkplaceTypes: []string{"remote", "hybrid"},
	}, "105104646", "São José dos Campos")
	if len(got) != 2 {
		t.Fatalf("%d buscas", len(got))
	}
	main := got[0]
	if main.Name != "Back-end · Go" {
		t.Errorf("nome = %q", main.Name)
	}
	want := `(Golang OR "Go developer" OR "Desenvolvedor Go" OR "Go engineer") AND (backend OR "back-end" OR "back end")`
	if main.Search.Keywords != want {
		t.Errorf("keywords =\n%s\nquer\n%s", main.Search.Keywords, want)
	}
	if !reflect.DeepEqual(main.Search.Experience, []string{"4"}) || !reflect.DeepEqual(main.Search.Workplace, []string{"2", "3"}) ||
		main.Search.GeoID != "105104646" {
		t.Errorf("filtros = %+v", main.Search)
	}
	// Nenhum filtro que a pessoa não pediu.
	if main.Search.PostedWithin != "" || main.Search.Sort != "" || main.Search.EasyApply || len(main.Search.JobTypes) != 0 {
		t.Errorf("filtro a mais: %+v", main.Search)
	}
	if got[1].Name != "Go (qualquer função)" || strings.Contains(got[1].Search.Keywords, "backend") {
		t.Errorf("busca larga = %+v", got[1])
	}
	if u := URL(main.Search); !strings.Contains(u, "keywords=%28Golang+OR") {
		t.Errorf("url = %s", u)
	}
}

func TestFromTargetOnlyOneSide(t *testing.T) {
	if got := FromTarget(discovery.Target{Technologies: []string{"Python"}}, "", ""); len(got) != 1 || got[0].Search.Keywords != "Python" {
		t.Errorf("só tecnologia = %+v", got)
	}
	if got := FromTarget(discovery.Target{Roles: []string{"data"}}, "", ""); len(got) != 1 || !strings.HasPrefix(got[0].Search.Keywords, "(dados OR") {
		t.Errorf("só função = %+v", got)
	}
	if got := FromTarget(discovery.Target{Seniorities: []string{"mid"}}, "", ""); got != nil {
		t.Errorf("Alvo sem tecnologia nem função gerou %+v", got)
	}
}

// Muitas tecnologias: a busca encolhe até caber em 200 caracteres.
func TestFromTargetFits(t *testing.T) {
	got := FromTarget(discovery.Target{
		Roles:        []string{"backend", "fullstack", "devops"},
		Technologies: []string{"Go", "Python", "Java", "TypeScript", "Node.js", "C#"},
	}, "", "")
	if len(got) != 2 {
		t.Fatalf("%d buscas (alguma passou do limite e sumiu)", len(got))
	}
	for _, g := range got {
		if n := len([]rune(g.Search.Keywords)); n > maxKeywords {
			t.Errorf("%q tem %d caracteres", g.Name, n)
		}
	}
}
