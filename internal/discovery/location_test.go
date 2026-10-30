package discovery

import "testing"

func TestMatchesLocation(t *testing.T) {
	sjc := LocationPrefs{WorkplaceTypes: []string{"remote", "hybrid"}, Cities: []string{"sao jose dos campos"}}
	cases := []struct {
		label     string
		prefs     LocationPrefs
		workplace string
		location  string
		intl      bool
		want      bool
	}{
		{"sem preferência passa tudo", LocationPrefs{}, "onsite", "Curitiba, Paraná", false, true},
		{"remota passa de qualquer lugar", sjc, "remote", "Curitiba, Paraná", false, true},
		{"híbrida na cidade, sem acento", sjc, "hybrid", "São José dos Campos, São Paulo", false, true},
		{"híbrida em outra cidade", sjc, "hybrid", "Curitiba, Paraná", false, false},
		{"presencial fora dos modos", sjc, "onsite", "São José dos Campos, São Paulo", false, false},
		{"só modo, sem cidade", LocationPrefs{WorkplaceTypes: []string{"remote"}}, "hybrid", "", false, false},
		{"modo desconhecido com cidade que bate", sjc, "", "São José dos Campos, SP", false, true},
		{"modo desconhecido sem cidade", sjc, "", "", false, false},
		{"só cidade: presencial em outra cidade", LocationPrefs{Cities: []string{"Jacareí"}}, "onsite", "Curitiba", false, false},
		{"só cidade: presencial na cidade", LocationPrefs{Cities: []string{"jacarei"}}, "onsite", "Jacareí, SP", false, true},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			if got := tc.prefs.MatchesLocation(tc.workplace, tc.location, tc.intl); got != tc.want {
				t.Errorf("MatchesLocation(%q, %q) = %v, quer %v", tc.workplace, tc.location, got, tc.want)
			}
		})
	}
}

func TestMatchesLocationAbroad(t *testing.T) {
	cases := []struct {
		label string
		prefs LocationPrefs
		intl  bool
		want  bool
	}{
		{"sem preferência mostra a internacional", LocationPrefs{}, true, true},
		{"só cidade não esconde a internacional", LocationPrefs{Cities: []string{"Jacareí"}}, true, true},
		{"remoto sem exterior esconde a internacional", LocationPrefs{WorkplaceTypes: []string{"remote"}}, true, false},
		{"exterior marcado mostra a internacional", LocationPrefs{WorkplaceTypes: []string{"remote", Abroad}}, true, true},
		{"só exterior esconde a nacional", LocationPrefs{WorkplaceTypes: []string{Abroad}}, false, false},
		{"exterior e remoto mantém a nacional remota", LocationPrefs{WorkplaceTypes: []string{Abroad, "remote"}}, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			if got := tc.prefs.MatchesLocation("remote", "", tc.intl); got != tc.want {
				t.Errorf("MatchesLocation(intl=%v) = %v, quer %v", tc.intl, got, tc.want)
			}
		})
	}
}
