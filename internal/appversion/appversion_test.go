package appversion

import (
	"testing"

	hacktontreino "hackton-treino"
)

// O version.json do repositório precisa ser válido: o deploy depende dele.
func TestVersionJSONDoRepositorio(t *testing.T) {
	if _, err := Parse(hacktontreino.VersionJSON); err != nil {
		t.Fatal(err)
	}
}

func TestParse(t *testing.T) {
	bad := []string{
		`{"version": "1.2", "date": "2026-10-31"}`,
		`{"version": "v1.2.3", "date": "2026-10-31"}`,
		`{"version": "1.2.3", "date": "31/10/2026"}`,
		`{"version": "1.2.3", "date": "2026-02-30"}`,
		`{"version": "1.2.3"}`,
		`nada`,
	}
	for _, raw := range bad {
		if _, err := Parse([]byte(raw)); err == nil {
			t.Errorf("Parse(%s) aceitou", raw)
		}
	}
	if info, err := Parse([]byte(`{"version": "1.10.0", "date": "2026-10-31"}`)); err != nil || info.Version != "1.10.0" {
		t.Errorf("Parse válido = %+v, %v", info, err)
	}
}

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.10.0", "1.9.0", 1}, // número a número, não texto
		{"0.2.0", "0.10.0", -1},
		{"2.0.0", "1.99.99", 1},
	}
	for _, c := range cases {
		if got := Compare(c.a, c.b); got != c.want {
			t.Errorf("Compare(%s, %s) = %d, quero %d", c.a, c.b, got, c.want)
		}
	}
}

func TestCheckRelease(t *testing.T) {
	live := Info{Version: "0.2.0", Date: "2026-10-31"}
	cases := []struct {
		next Info
		ok   bool
	}{
		{Info{"0.2.1", "2026-10-31"}, true},  // mesmo dia, versão maior
		{Info{"0.3.0", "2026-11-02"}, true},  // dia seguinte
		{Info{"0.2.0", "2026-11-01"}, false}, // mesma versão
		{Info{"0.1.9", "2026-11-01"}, false}, // versão menor
		{Info{"0.2.1", "2026-10-30"}, false}, // dia anterior
	}
	for _, c := range cases {
		if err := CheckRelease(live, c.next); (err == nil) != c.ok {
			t.Errorf("CheckRelease(%+v) = %v, quero ok=%v", c.next, err, c.ok)
		}
	}
}
