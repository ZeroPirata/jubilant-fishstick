package discovery

import (
	"slices"
	"testing"
)

func TestParseBoardURL(t *testing.T) {
	cases := map[string][2]string{
		"https://boards.greenhouse.io/stone":                    {"greenhouse", "stone"},
		"job-boards.greenhouse.io/c6bank/jobs/4739497005":       {"greenhouse", "c6bank"},
		"https://boards-api.greenhouse.io/v1/boards/xpinc/jobs": {"greenhouse", "xpinc"},
		"https://jobs.lever.co/CIANDT/513de4fc":                 {"lever", "ciandt"},
	}
	for in, want := range cases {
		src, tok, ok := parseBoardURL(in)
		if !ok || src != want[0] || tok != want[1] {
			t.Errorf("parseBoardURL(%q) = %q, %q, %v; quer %v", in, src, tok, ok, want)
		}
	}
	for _, in := range []string{"Banco Inter", "https://www.gupy.io/stone", "stone"} {
		if _, _, ok := parseBoardURL(in); ok {
			t.Errorf("%q não é link de quadro de vagas", in)
		}
	}
}

func TestTokenCandidates(t *testing.T) {
	got := tokenCandidates("Banco Inter S.A.")
	for _, want := range []string{"bancointer", "banco-inter", "banco", "inter"} {
		if !slices.Contains(got, want) {
			t.Errorf("faltou %q em %v", want, got)
		}
	}
	if got := tokenCandidates("Stone"); !slices.Equal(got, []string{"stone"}) {
		t.Errorf("nome de uma palavra: %v", got)
	}
	if got := tokenCandidates("São Paulo Tech"); got[0] != "saopaulotech" {
		t.Errorf("acentos: %v", got)
	}
}
