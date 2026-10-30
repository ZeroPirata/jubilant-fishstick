package discovery

import (
	"encoding/json"
	"testing"
)

func decode[T any](t *testing.T, raw string) T {
	t.Helper()
	var v T
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestParseAshby(t *testing.T) {
	board := decode[ashbyBoard](t, `{"jobs": [
	  {"id": "a1", "title": " Software Engineer ", "location": "São Paulo", "isListed": true, "isRemote": true,
	   "workplaceType": "Hybrid", "publishedAt": "2026-10-28T17:26:58.786+00:00",
	   "address": {"postalAddress": {"addressLocality": "São Paulo", "addressCountry": "Brazil"}},
	   "jobUrl": "https://jobs.ashbyhq.com/nubank/a1", "descriptionPlain": "Go e Kafka"},
	  {"id": "a2", "title": "Lead Analyst", "location": "Buenos Aires", "isListed": true, "isRemote": true,
	   "workplaceType": "Hybrid", "address": {"postalAddress": {"addressCountry": "Argentina"}},
	   "jobUrl": "https://jobs.ashbyhq.com/nubank/a2"},
	  {"id": "a3", "title": "Escondida", "location": "São Paulo", "isListed": false,
	   "jobUrl": "https://jobs.ashbyhq.com/nubank/a3"},
	  {"id": "a4", "title": "Remote dev", "location": "Remote", "isListed": true, "isRemote": true,
	   "workplaceType": "Remote", "secondaryLocations": [{"location": "LATAM"}],
	   "jobUrl": "https://jobs.ashbyhq.com/nubank/a4"}
	]}`)
	got := parseAshby(Board{Token: "nubank", Company: "Nubank"}, board.Jobs)
	if len(got) != 2 {
		t.Fatalf("vagas = %d, quero 2 (Brasil e remota LATAM; Argentina e isListed=false fora): %+v", len(got), got)
	}
	br := got[0]
	if br.Title != "Software Engineer" || br.WorkplaceType != "hybrid" || br.International || br.Company != "Nubank" ||
		br.ExternalID != "nubank/a1" || br.PublishedAt.IsZero() || br.Description != "Go e Kafka" {
		t.Errorf("vaga no Brasil = %+v", br)
	}
	if r := got[1]; r.WorkplaceType != "remote" || !r.International {
		t.Errorf("remota LATAM = %+v", r)
	}
}

func TestParseWorkable(t *testing.T) {
	acc := decode[workableAccount](t, `{"name": "Acme", "jobs": [
	  {"title": "Backend Developer", "shortcode": "AB12", "telecommuting": false,
	   "shortlink": "https://apply.workable.com/j/AB12", "published_on": "2026-10-20",
	   "country": "Brazil", "city": "Curitiba", "state": "Paraná", "description": "<p>Python</p>"},
	  {"title": "Account Manager", "shortcode": "CD34", "shortlink": "https://apply.workable.com/j/CD34",
	   "country": "Greece", "city": "Athens", "locations": [{"country": "Greece", "city": "Athens"}]}
	]}`)
	got := parseWorkable(Board{Token: "acme"}, acc)
	if len(got) != 1 {
		t.Fatalf("vagas = %d, quero 1: %+v", len(got), got)
	}
	j := got[0]
	if j.Company != "Acme" || j.URL != "https://apply.workable.com/j/AB12" || j.Description != "Python" ||
		j.PublishedAt.Format("2006-01-02") != "2026-10-20" || j.ExternalID != "acme/AB12" {
		t.Errorf("vaga = %+v", j)
	}
}

func TestParseRecruitee(t *testing.T) {
	page := decode[recruiteeOffers](t, `{"offers": [
	  {"id": 1, "title": "Dev Go", "company_name": "Bunq", "careers_url": "https://x.recruitee.com/o/dev-go",
	   "location": "São Paulo, Brazil", "status": "published", "hybrid": true,
	   "published_at": "2026-10-28 13:21:58 UTC", "description": "<p>Go</p>", "requirements": "<ul><li>SQL</li></ul>"},
	  {"id": 2, "title": "Data Scientist", "careers_url": "https://x.recruitee.com/o/ds",
	   "location": "Amsterdam, Netherlands", "status": "published"}
	]}`)
	got := parseRecruitee(Board{Token: "bunq"}, page.Offers)
	if len(got) != 1 {
		t.Fatalf("vagas = %d, quero 1: %+v", len(got), got)
	}
	j := got[0]
	if j.WorkplaceType != "hybrid" || j.Company != "Bunq" || j.PublishedAt.IsZero() || j.Description != "Go\n\nSQL" {
		t.Errorf("vaga = %+v (descrição %q)", j, j.Description)
	}
}

func TestParseSmartRecruiters(t *testing.T) {
	d := decode[smartRecruitersDetail](t, `{"id": "744", "name": "Engenheiro de Dados",
	  "company": {"name": "Experian"}, "releasedDate": "2026-10-30T21:16:07.046Z",
	  "location": {"city": "São Paulo", "country": "br", "hybrid": true, "fullLocation": "São Paulo, , Brazil"},
	  "postingUrl": "https://jobs.smartrecruiters.com/Experian/744-engenheiro",
	  "jobAd": {"sections": {"jobDescription": {"text": "<p>Spark</p>"}, "qualifications": {"text": "<p>Python</p>"}}}}`)
	j, ok := parseSmartRecruiters(Board{Token: "experian"}, d)
	if !ok {
		t.Fatal("vaga no Brasil descartada")
	}
	if j.WorkplaceType != "hybrid" || j.Company != "Experian" || j.Location != "São Paulo" ||
		j.Description != "Spark\n\nPython" || j.PublishedAt.IsZero() || j.International {
		t.Errorf("vaga = %+v", j)
	}
	if _, ok := parseSmartRecruiters(Board{}, smartRecruitersDetail{}); ok {
		t.Error("vaga sem link aceita")
	}
}

func TestParseBoardURL_NovosATS(t *testing.T) {
	cases := map[string][2]string{
		"https://jobs.ashbyhq.com/nubank":                   {"ashby", "nubank"},
		"https://jobs.ashbyhq.com/nubank/7f8653ae":          {"ashby", "nubank"},
		"apply.workable.com/olist/":                         {"workable", "olist"},
		"https://jobs.smartrecruiters.com/Experian/744-eng": {"smartrecruiters", "experian"},
		"https://bunq.recruitee.com/o/data-scientist":       {"recruitee", "bunq"},
	}
	for raw, want := range cases {
		src, tok, ok := parseBoardURL(raw)
		if !ok || src != want[0] || tok != want[1] {
			t.Errorf("parseBoardURL(%q) = %q, %q, %v; quero %v", raw, src, tok, ok, want)
		}
	}
	if _, _, ok := parseBoardURL("https://www.recruitee.com/pricing"); ok {
		t.Error("site do Recruitee tratado como empresa")
	}
}
