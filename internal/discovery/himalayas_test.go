package discovery

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseHimalayas(t *testing.T) {
	raw := `{"offset": 0, "totalCount": 21, "jobs": [
	  {"guid": "https://himalayas.app/companies/canonical/jobs/golang-engineer", "title": "Golang Engineer",
	   "companyName": "Canonical", "description": "<p>We use <b>Go</b> and Linux.</p>",
	   "applicationLink": "https://himalayas.app/companies/canonical/jobs/golang-engineer?utm_source=api",
	   "locationRestrictions": [], "categories": ["Golang-Development"], "pubDate": 1789842948, "expiryDate": 1795026947},
	  {"guid": "g2", "title": "Backend Dev", "companyName": "Acme", "description": "", "applicationLink": "https://himalayas.app/x",
	   "locationRestrictions": ["Brazil", "Argentina", "Chile", "Peru"], "pubDate": 0, "expiryDate": 0},
	  {"guid": "", "title": "sem id", "applicationLink": "https://himalayas.app/y"}
	]}`
	var p himalayasPage
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatal(err)
	}
	got, more := parseHimalayas(p)
	if !more || len(got) != 2 {
		t.Fatalf("esperava 2 vagas e mais páginas: %v %d", more, len(got))
	}

	j := got[0]
	if j.URL != "https://himalayas.app/companies/canonical/jobs/golang-engineer?utm_source=api" {
		t.Errorf("a candidatura precisa usar o applicationLink: %q", j.URL)
	}
	if j.WorkplaceType != "remote" || j.Location != "qualquer país" || j.Company != "Canonical" {
		t.Errorf("campos errados: %+v", j)
	}
	if strings.Contains(j.Description, "<p>") || !strings.Contains(j.Description, "Golang Development") {
		t.Errorf("descrição devia vir em texto, com as categorias: %q", j.Description)
	}
	if j.PublishedAt.IsZero() || j.ExpiresAt.Before(j.PublishedAt) {
		t.Errorf("datas: %v %v", j.PublishedAt, j.ExpiresAt)
	}
	if got[1].Location != "4 países, incluindo Brasil" || !got[1].PublishedAt.IsZero() {
		t.Errorf("segunda vaga: %+v", got[1])
	}
}

func TestHimalayasInternational(t *testing.T) {
	page := himalayasPage{TotalCount: 3, Jobs: []himalayasJob{
		{GUID: "a", ApplicationLink: "https://h/a"},
		{GUID: "b", ApplicationLink: "https://h/b", LocationRestrictions: []string{"Brazil", "Chile"}},
		{GUID: "c", ApplicationLink: "https://h/c", LocationRestrictions: []string{"Brazil"}},
	}}
	got, _ := parseHimalayas(page)
	if !got[0].International || !got[1].International {
		t.Error("vaga aberta a outros países é do exterior")
	}
	if got[2].International || got[2].Location != "Brasil" {
		t.Errorf("vaga só para o Brasil é remota nacional: %+v", got[2])
	}
}
