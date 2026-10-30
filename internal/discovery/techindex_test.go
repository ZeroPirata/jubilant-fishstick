package discovery

import (
	"slices"
	"testing"
	"time"
)

func TestIndexJob_RegrasDeContexto(t *testing.T) {
	idx := IndexJob("Junior Media Buyer", "We are a go-to partner for brands. Experience with Google Ads.")
	if slices.Contains(idx.Techs, "Go") {
		t.Errorf("techs = %v: \"go-to\" não é Go", idx.Techs)
	}
	idx = IndexJob("Backend Developer", "Stack: Go, PostgreSQL e Docker. You will work with our team on the platform.")
	for _, want := range []string{"Go", "PostgreSQL", "Docker"} {
		if !slices.Contains(idx.Techs, want) {
			t.Errorf("techs = %v, falta %s", idx.Techs, want)
		}
	}
	if idx.Techs == nil {
		t.Error("techs nunca é nil (coluna NOT NULL)")
	}
}

func TestTechsVersion(t *testing.T) {
	if TechsVersion <= 0 {
		t.Errorf("TechsVersion = %d, want positivo (0 é \"nunca calculada\")", TechsVersion)
	}
}

func TestPlanSkills(t *testing.T) {
	skills := []Skill{
		NewSkill("Go (Golang)"),
		NewSkill("Fiber (Go)"),
		NewSkill("APIs REST"),
		NewSkill("Arquitetura Hexagonal"),
		NewSkill("UDP / CRC-16"),
		NewSkill("C#"),
	}
	p := PlanSkills(skills)

	if !slices.Contains(p.AllTechs, "Go") || !slices.Contains(p.AllTechs, "C#") {
		t.Errorf("AllTechs = %v, want Go e C#", p.AllTechs)
	}
	for _, want := range []string{"fiber", "apis rest", "arquitetura hexagonal", "udp", "crc-16"} {
		if !slices.Contains(p.Terms, want) {
			t.Errorf("Terms = %v, falta %q", p.Terms, want)
		}
	}
	// Coberto pelo catálogo, ambíguo ou com símbolo não vai para a busca de texto.
	for _, no := range []string{"go", "golang", "c#", "go (golang)", "udp / crc-16"} {
		if slices.Contains(p.Terms, no) {
			t.Errorf("Terms = %v, não deveria ter %q", p.Terms, no)
		}
	}

	got := p.Matched([]string{"Go", "Docker"}, []string{"apis rest"})
	want := []string{"Go (Golang)", "Fiber (Go)", "APIs REST"}
	if !slices.Equal(got, want) {
		t.Errorf("Matched = %v, want %v", got, want)
	}
	if got := p.Matched(nil, []string{"crc-16"}); !slices.Equal(got, []string{"UDP / CRC-16"}) {
		t.Errorf("Matched por termo = %v", got)
	}
	if got := p.Matched([]string{"Python"}, nil); len(got) != 0 {
		t.Errorf("Matched = %v, want vazio", got)
	}
}

func TestTooOld(t *testing.T) {
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		pub  time.Time
		want bool
	}{
		{"sem data", time.Time{}, false},
		{"recente", now.Add(-24 * time.Hour), false},
		{"no limite", now.Add(-MaxJobAge), false},
		{"velha", now.Add(-MaxJobAge - time.Hour), true},
	}
	for _, c := range cases {
		if got := tooOld(Listing{PublishedAt: c.pub}, now); got != c.want {
			t.Errorf("%s: tooOld = %v, quero %v", c.name, got, c.want)
		}
	}
}
