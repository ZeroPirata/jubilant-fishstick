package linkedinsearch

import (
	"reflect"
	"testing"
)

func TestURLEmpty(t *testing.T) {
	if got := URL(Search{}); got != "https://www.linkedin.com/jobs/search/" {
		t.Errorf("busca vazia = %q", got)
	}
}

func TestURLAllFilters(t *testing.T) {
	s, err := Clean(Search{
		Keywords: "  Desenvolvedor   Back-end Pleno ", GeoID: "105104646", GeoLabel: "São José dos Campos",
		PostedWithin: "r604800", Workplace: []string{"3", "2", "3"}, Experience: []string{"4", "3"},
		JobTypes: []string{"F"}, EasyApply: true, FewApplicants: true, InNetwork: true, Sort: "DD",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "https://www.linkedin.com/jobs/search/?f_AL=true&f_E=3%2C4&f_EA=true&f_JIYN=true&f_JT=F&f_TPR=r604800&f_WT=2%2C3&geoId=105104646&keywords=Desenvolvedor+Back-end+Pleno&sortBy=DD"
	if got := URL(s); got != want {
		t.Errorf("url =\n%s\nquer\n%s", got, want)
	}
}

func TestCleanRejects(t *testing.T) {
	for name, s := range map[string]Search{
		"período":  {PostedWithin: "r1"},
		"modelo":   {Workplace: []string{"9"}},
		"nível":    {Experience: []string{"7"}},
		"tipo":     {JobTypes: []string{"X"}},
		"ordem":    {Sort: "Z"},
		"local":    {GeoID: "abc"},
		"keywords": {Keywords: string(make([]byte, 201))},
	} {
		if _, err := Clean(s); err == nil {
			t.Errorf("%s: aceitou %+v", name, s)
		}
	}
}

func TestCleanDropsLabelWithoutGeo(t *testing.T) {
	s, _ := Clean(Search{GeoLabel: "Lugar"})
	if s.GeoLabel != "" {
		t.Errorf("rótulo sem local = %q", s.GeoLabel)
	}
}

// A URL do print: busca nova (search-results), com vaga aberta e rastreio.
func TestFromURLSearchResults(t *testing.T) {
	s, err := FromURL("https://www.linkedin.com/jobs/search-results/?currentJobId=4490508535&showHowYouFit=HOW_YOU_FIT&keywords=Desenvolvedor%20Back-end%20Pleno&origin=QUALIFICATION_LANDING&originToLandingJobPostings=4490508535%2C4490511393&geoId=105104646")
	if err != nil {
		t.Fatal(err)
	}
	want := Search{Keywords: "Desenvolvedor Back-end Pleno", GeoID: "105104646", Workplace: []string{}, Experience: []string{}, JobTypes: []string{}}
	if !reflect.DeepEqual(s, want) {
		t.Errorf("busca = %+v", s)
	}
}

func TestFromURLFilters(t *testing.T) {
	s, err := FromURL("https://linkedin.com/jobs/search/?keywords=go&geoId=106057199&f_TPR=r86400&f_WT=2%2C9&f_E=2&f_E=3&f_JT=F,P&f_AL=true&sortBy=DD&trk=x")
	if err != nil {
		t.Fatal(err)
	}
	if s.GeoLabel != "Brasil" || s.PostedWithin != "r86400" || !s.EasyApply || s.Sort != "DD" {
		t.Errorf("busca = %+v", s)
	}
	if !reflect.DeepEqual(s.Workplace, []string{"2"}) || !reflect.DeepEqual(s.Experience, []string{"2", "3"}) || !reflect.DeepEqual(s.JobTypes, []string{"F", "P"}) {
		t.Errorf("listas = %v %v %v", s.Workplace, s.Experience, s.JobTypes)
	}
	// Ida e volta: a URL gerada lê de novo a mesma busca.
	again, err := FromURL(URL(s))
	if err != nil || !reflect.DeepEqual(again, s) {
		t.Errorf("ida e volta = %+v (%v)", again, err)
	}
}

func TestFromURLRejects(t *testing.T) {
	for _, raw := range []string{
		"", "linkedin.com/jobs/search/", "https://evil.com/jobs/search/?keywords=go",
		"https://linkedin.com.evil.com/jobs/search/", "https://www.linkedin.com/feed/",
		"javascript:alert(1)",
	} {
		if _, err := FromURL(raw); err == nil {
			t.Errorf("aceitou %q", raw)
		}
	}
}
