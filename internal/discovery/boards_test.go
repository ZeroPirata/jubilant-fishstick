package discovery

import (
	"strings"
	"testing"
)

func TestClassifyPlace(t *testing.T) {
	cases := []struct {
		loc        string
		remoteHint bool
		workplace  string
		intl, keep bool
	}{
		{"São Paulo, Brazil", false, "", false, true},
		{"Brazil (Remote)", false, "remote", false, true},
		{"Brazil (São Paulo - Hybrid)", false, "hybrid", false, true},
		{"Remote - LATAM", false, "remote", true, true},
		{"Worldwide", true, "remote", true, true},
		{"US (Remote)", false, "", false, false},
		{"Germany (Berlin - Hybrid)", false, "", false, false},
		{"Mexico City", false, "", false, false},
		{"Spain", false, "", false, false}, // "sp" não é marcador, e "spain" não é Brasil
	}
	for _, tc := range cases {
		wt, intl, keep := ClassifyPlace(tc.loc, tc.remoteHint)
		if keep != tc.keep || (keep && (wt != tc.workplace || intl != tc.intl)) {
			t.Errorf("ClassifyPlace(%q) = %q, %v, %v; quer %q, %v, %v", tc.loc, wt, intl, keep, tc.workplace, tc.intl, tc.keep)
		}
	}
}

func TestParseGreenhouse(t *testing.T) {
	jobs := []greenhouseJob{
		{ID: 1, Title: " Dev Go ", AbsoluteURL: "https://job-boards.greenhouse.io/c6bank/jobs/1",
			Content: "&lt;p&gt;Usamos &lt;b&gt;Go&lt;/b&gt;&lt;/p&gt;", FirstPub: "2026-09-30T17:37:05-04:00"},
		{ID: 2, Title: "Engineer", AbsoluteURL: "https://x/2"},
	}
	jobs[0].Location.Name = "São Paulo, Brazil"
	jobs[1].Location.Name = "US (Remote)"

	got := parseGreenhouse(Board{Token: "c6bank", Company: "C6 Bank"}, jobs)
	if len(got) != 1 {
		t.Fatalf("vaga dos EUA deveria sair: %+v", got)
	}
	j := got[0]
	if j.ExternalID != "c6bank/1" || j.Company != "C6 Bank" || j.Title != "Dev Go" || j.PublishedAt.IsZero() {
		t.Errorf("campos: %+v", j)
	}
	if strings.Contains(j.Description, "<") || !strings.Contains(j.Description, "Usamos Go") {
		t.Errorf("descrição devia vir em texto: %q", j.Description)
	}
}

func TestParseLever(t *testing.T) {
	ps := []leverPosting{
		{ID: "a", Text: "[ job -  32032] Data PM", HostedURL: "https://jobs.lever.co/ciandt/a", Country: "BR", WorkplaceType: "hybrid",
			DescriptionPlain: "Sobre", CreatedAt: 1790779378271},
		{ID: "b", Text: "PM Colombia", HostedURL: "https://jobs.lever.co/ciandt/b", Country: "CO", WorkplaceType: "remote"},
		{ID: "c", Text: "Dev LATAM", HostedURL: "https://jobs.lever.co/ciandt/c", Country: "US", WorkplaceType: "remote"},
	}
	ps[0].Categories.Location = "Campinas, SP"
	ps[1].Categories.Location = "Colombia"
	ps[2].Categories.Location = "LATAM"
	ps[0].Lists = append(ps[0].Lists, struct {
		Text    string `json:"text"`
		Content string `json:"content"`
	}{"Requisitos", "<li>Python</li>"})

	got := parseLever(Board{Token: "ciandt", Company: "CI&T"}, ps)
	if len(got) != 2 {
		t.Fatalf("remota só para a Colômbia deveria sair: %+v", got)
	}
	if got[0].Title != "Data PM" || got[0].WorkplaceType != "hybrid" || got[0].International || !strings.Contains(got[0].Description, "Python") || got[0].PublishedAt.IsZero() {
		t.Errorf("vaga brasileira: %+v", got[0])
	}
	if !got[1].International || got[1].WorkplaceType != "remote" {
		t.Errorf("remota LATAM é do exterior: %+v", got[1])
	}
}
