package changelog

import (
	"strings"
	"testing"

	hacktontreino "hackton-treino"
	"hackton-treino/internal/appversion"
)

func TestParse(t *testing.T) {
	intro, list := Parse("# Changelog\n\nO que **mudou**.\n\n## 0.2.0 (31/10/2026)\nResumo.\n\n- um `version.json`\n- dois\n  continua\n\n## Antes da 0.2.0\nTexto.")
	if intro != "<p>O que <strong>mudou</strong>.</p>" {
		t.Errorf("intro = %q", intro)
	}
	if len(list) != 2 {
		t.Fatalf("%d versões", len(list))
	}
	if list[0].Version != "0.2.0" || list[0].Date != "31/10/2026" {
		t.Errorf("versão = %q %q", list[0].Version, list[0].Date)
	}
	if list[0].HTML != "<p>Resumo.</p><ul><li>um <code>version.json</code></li><li>dois continua</li></ul>" {
		t.Errorf("html = %q", list[0].HTML)
	}
	if list[1].Version != "Antes da 0.2.0" || list[1].Date != "" {
		t.Errorf("sem data = %q %q", list[1].Version, list[1].Date)
	}
}

// Texto vira texto: nada do arquivo vira tag, e link só para http(s) ou "#/".
func TestRenderEscapes(t *testing.T) {
	got := Render("<script>x</script> [a](javascript:alert(1)) [b](https://ex.com/?a=1&b=2) [c](#/admin)")
	for _, bad := range []string{"<script>", "javascript:"} {
		if strings.Contains(got, bad) {
			t.Errorf("%q passou: %s", bad, got)
		}
	}
	for _, want := range []string{`<a href="https://ex.com/?a=1&amp;b=2" target="_blank" rel="noopener noreferrer">b</a>`, `<a href="#/admin">c</a>`} {
		if !strings.Contains(got, want) {
			t.Errorf("falta %s em %s", want, got)
		}
	}
}

// A versão do version.json precisa estar no CHANGELOG.md (o checkversion barra o
// deploy; aqui o go test avisa antes).
func TestCurrentVersionHasEntry(t *testing.T) {
	v := appversion.Current().Version
	if !HasVersion(string(hacktontreino.ChangelogMD), v) {
		t.Errorf("CHANGELOG.md sem a entrada \"## %s (DD/MM/AAAA)\"", v)
	}
	_, list := Parse(string(hacktontreino.ChangelogMD))
	if len(list) == 0 || list[0].Version != v {
		t.Errorf("a primeira entrada do CHANGELOG.md deve ser a versão atual (%s)", v)
	}
}
