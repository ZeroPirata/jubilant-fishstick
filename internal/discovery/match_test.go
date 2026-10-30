package discovery

import (
	"reflect"
	"regexp"
	"testing"
)

func TestNewSkill(t *testing.T) {
	cases := []struct {
		name string
		want []string
	}{
		{"Go (Golang)", []string{"go (golang)", "go", "golang"}},
		{"Concorrência / Goroutines", []string{"concorrência / goroutines", "concorrência", "goroutines"}},
		{"Otimização de Performance (BD)", []string{"otimização de performance (bd)", "otimização de performance", "bd"}},
		{"Redis", []string{"redis"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NewSkill(tc.name).Terms
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Terms = %q, quer %q", got, tc.want)
			}
		})
	}
}

func skills(names ...string) []Skill {
	out := make([]Skill, len(names))
	for i, n := range names {
		out[i] = NewSkill(n)
	}
	return out
}

func TestMatchSkills(t *testing.T) {
	cases := []struct {
		label  string
		text   string
		skills []Skill
		want   []string
	}{
		{"palavra inteira", "Experiência com Go e PostgreSQL", skills("Go", "PostgreSQL"), []string{"Go", "PostgreSQL"}},
		{"go não bate em google", "Trabalhamos com Google Cloud", skills("go"), nil},
		{"c não bate em c# nem c++", "Vaga para C# e C++", skills("c"), nil},
		{"c# bate", "Vaga para C# sênior", skills("C#"), []string{"C#"}},
		{"node bate em node.js", "Back-end em Node.js", skills("node"), []string{"node"}},
		{"js não bate em node.js", "Back-end em Node.js", skills("js"), nil},
		{"fim de frase", "Usamos Go.", skills("go"), []string{"go"}},
		{"acento conta como letra", "programação", skills("programa"), nil},
		{"skill repetida conta uma vez", "react", skills("React", "react"), []string{"React"}},
		{"segunda ocorrência válida", "golang e go", skills("go"), []string{"go"}},
		{"variação do nome", "Serviços em Golang", skills("Go (Golang)"), []string{"Go (Golang)"}},
		{"parte depois da barra", "Uso de goroutines", skills("Concorrência / Goroutines"), []string{"Concorrência / Goroutines"}},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			got := MatchSkills(tc.text, tc.skills)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("MatchSkills(%q) = %v, quer %v", tc.text, got, tc.want)
			}
		})
	}
}

// O regex vai para o Postgres, mas a sintaxe usada ([[:alnum:]], QuoteMeta) é a mesma
// no RE2: dá para conferir aqui que ele é mais largo que MatchSkills, nunca mais estreito.
func TestRegexPattern(t *testing.T) {
	re := regexp.MustCompile("(?i)" + RegexPattern(skills("C++", "go")))
	match := re.MatchString
	for _, text := range []string{"Vaga C++ sênior", "usamos Go.", "go"} {
		if !match(text) {
			t.Errorf("deveria passar no filtro do banco: %q", text)
		}
	}
	if match("Google e algoritmos") {
		t.Error("go não deveria passar em google/algoritmos")
	}
}
