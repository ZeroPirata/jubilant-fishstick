package handler

import (
	"hackton-treino/internal/db"
	"hackton-treino/internal/discovery"
	"hackton-treino/internal/jobfit"
	"slices"
	"testing"
)

func names(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, quer %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got %v, quer %v", got, want)
		}
	}
}

func TestStrongSkills(t *testing.T) {
	skills := []db.UserSkill{
		{SkillName: "Go", ProficiencyLevel: db.SkillLevelExpert},
		{SkillName: "Java", ProficiencyLevel: db.SkillLevelBasic},
		{SkillName: "Redis", ProficiencyLevel: db.SkillLevelAdvanced},
		{SkillName: "Rust", ProficiencyLevel: db.SkillLevelIntermediate},
	}
	got, onlyStrong := strongSkills(skills)
	if !onlyStrong {
		t.Error("com skills avançadas/expert, onlyStrong deveria ser true")
	}
	var gotNames []string
	for _, s := range got {
		gotNames = append(gotNames, s.Name)
	}
	names(t, gotNames, "Go", "Redis")

	weak := []db.UserSkill{{SkillName: "Java", ProficiencyLevel: db.SkillLevelBasic}}
	got, onlyStrong = strongSkills(weak)
	if onlyStrong || len(got) != 1 || got[0].Name != "Java" {
		t.Errorf("sem skill forte deveria cair para todas: %v, %v", got, onlyStrong)
	}
}

func TestFilterDiscovered(t *testing.T) {
	jobs := []DiscoveredJob{
		{ID: "1", Source: "gupy", WorkplaceType: "remote", MatchedSkills: []string{"Go", "Redis"}},
		{ID: "2", Source: "gupy", WorkplaceType: "hybrid", Location: "Curitiba, Paraná", MatchedSkills: []string{"Go"}},
		{ID: "3", Source: "linkedin", WorkplaceType: "hybrid", Location: "São José dos Campos, SP", MatchedSkills: []string{"Go", "gRPC"}},
		{ID: "4", Source: "linkedin", WorkplaceType: "onsite", Location: "São José dos Campos, SP", MatchedSkills: []string{"Redis"}},
		{ID: "5", Source: "himalayas", WorkplaceType: "remote", International: true, MatchedSkills: []string{"Go"}},
	}
	loc := discovery.LocationPrefs{WorkplaceTypes: []string{"remote", "hybrid"}, Cities: []string{"São José dos Campos"}}

	ids := func(js []DiscoveredJob) []string {
		var out []string
		for _, j := range js {
			out = append(out, j.ID)
		}
		return out
	}

	res := filterDiscovered(jobs, discoveryFilter{Location: loc})
	names(t, ids(res.Jobs), "1", "3")

	// A vaga internacional só entra com "Exterior" marcado.
	abroad := discovery.LocationPrefs{WorkplaceTypes: []string{"remote", discovery.Abroad}}
	names(t, ids(filterDiscovered(jobs, discoveryFilter{Location: abroad}).Jobs), "1", "5")

	res = filterDiscovered(jobs, discoveryFilter{Location: loc, Sources: []string{"linkedin"}})
	names(t, ids(res.Jobs), "3")
	// A contagem de plataformas ignora o próprio filtro de plataforma.
	if len(res.Facets.Sources) != 2 {
		t.Errorf("facets de plataforma deveriam listar gupy e linkedin: %+v", res.Facets.Sources)
	}

	res = filterDiscovered(jobs, discoveryFilter{Location: loc, Skills: []string{"Go", "Redis"}})
	names(t, ids(res.Jobs), "1")
	// Contagem de skills considera o local: Redis só aparece na vaga 1 (a 4 é presencial).
	for _, f := range res.Facets.Skills {
		if f.Name == "Redis" && f.Count != 1 {
			t.Errorf("Redis deveria contar 1, contou %d", f.Count)
		}
	}
}

func TestCleanPreferences(t *testing.T) {
	got, err := cleanPreferences(DiscoveryPreferences{
		WorkplaceTypes: []string{"remote", "hybrid", "remote"},
		Cities:         []string{"  São José   dos Campos ", "", "São José dos Campos"},
	})
	if err != nil {
		t.Fatal(err)
	}
	names(t, got.WorkplaceTypes, "remote", "hybrid")
	names(t, got.Cities, "São José dos Campos")

	if got, err := cleanPreferences(DiscoveryPreferences{WorkplaceTypes: []string{"abroad"}}); err != nil || len(got.WorkplaceTypes) != 1 {
		t.Errorf("exterior é uma opção válida: %v %v", got, err)
	}
	got, err = cleanPreferences(DiscoveryPreferences{Technologies: []string{"Go", "Kubernetes", "Go"}})
	if err != nil {
		t.Fatal(err)
	}
	names(t, got.Technologies, "Go", "Kubernetes")
	if _, err := cleanPreferences(DiscoveryPreferences{Technologies: []string{"golang"}}); err == nil {
		t.Error("tecnologia fora do catálogo deveria dar erro (o catálogo usa o nome, \"Go\")")
	}
	if _, err := cleanPreferences(DiscoveryPreferences{WorkplaceTypes: []string{"mars"}}); err == nil {
		t.Error("modo desconhecido deveria dar erro")
	}
}

func TestCleanDiscoverySettings(t *testing.T) {
	got, err := cleanDiscoverySettings(DiscoverySettingsBody{
		Enabled:         true,
		IntervalMinutes: 360,
		CatalogTerms:    []string{"  Desenvolvedor  Backend ", "desenvolvedor backend", "", "DevOps"},
		DisabledSources: []string{"gupy", "gupy"},
		Boards: map[string][]discovery.Board{"greenhouse": {
			{Token: " Stone ", Company: "Stone"}, {Token: "stone"}, {Token: "c6bank"},
		}},
	}, testSources)
	if err != nil {
		t.Fatal(err)
	}
	names(t, got.CatalogTerms, "desenvolvedor backend", "devops")
	names(t, got.DisabledSources, "gupy")
	if gh := got.Boards["greenhouse"]; len(gh) != 2 || gh[0] != (discovery.Board{Token: "stone", Company: "Stone"}) || gh[1].Company != "c6bank" {
		t.Errorf("empresas: %+v", got.Boards)
	}

	bad := []map[string][]discovery.Board{
		{"greenhouse": {{Token: "https://boards.greenhouse.io/stone"}}}, // URL em vez do token
		{"gupy": {{Token: "stone"}}},                                    // gupy não usa empresas
	}
	for _, b := range bad {
		if _, err := cleanDiscoverySettings(DiscoverySettingsBody{IntervalMinutes: 60, Boards: b}, testSources); err == nil {
			t.Errorf("empresas %+v deveriam ser recusadas", b)
		}
	}

	for _, minutes := range []int{0, 14, 7*24*60 + 1} {
		if _, err := cleanDiscoverySettings(DiscoverySettingsBody{IntervalMinutes: minutes}, nil); err == nil {
			t.Errorf("intervalo de %d min deveria ser recusado", minutes)
		}
	}
	if _, err := cleanDiscoverySettings(DiscoverySettingsBody{IntervalMinutes: 60, DisabledSources: []string{"indeed"}}, testSources); err == nil {
		t.Error("site não registrado deveria ser recusado")
	}
}

var testSources = []discovery.SourceInfo{
	{Name: "gupy", Kind: discovery.KindSearch},
	{Name: "linkedin", Kind: discovery.KindSearch},
	{Name: "greenhouse", Kind: discovery.KindList, UsesBoards: true},
}

func TestBuildSourceStatus(t *testing.T) {
	got := buildSourceStatus(
		testSources[:2],
		[]string{"linkedin"},
		[]db.DiscoveryCatalogStatsRow{{Source: "gupy", Total: 10, Active: 8}, {Source: "catho", Total: 3}},
		[]db.DiscoveryLastSourceResultsRow{{Source: "gupy", KeywordsOk: 5, KeywordsFailed: 1, LastError: "timeout"}},
	)
	if len(got) != 3 {
		t.Fatalf("esperava gupy, linkedin e catho (só no histórico): %+v", got)
	}
	gupy, linkedin, catho := got[0], got[1], got[2]
	if !gupy.Enabled || gupy.JobsActive != 8 || gupy.KeywordsFailed != 1 || gupy.LastError != "timeout" {
		t.Errorf("gupy: %+v", gupy)
	}
	if linkedin.Enabled {
		t.Error("linkedin está desligado")
	}
	if catho.Name != "catho" || catho.Enabled || catho.Registered {
		t.Errorf("fonte fora do código não pode aparecer ligada: %+v", catho)
	}
}

func TestFilterDiscovered_Nivel(t *testing.T) {
	jobs := []DiscoveredJob{
		{ID: "1", Source: "gupy", Seniority: []string{"mid"}},
		{ID: "2", Source: "gupy", Seniority: []string{"senior"}},
		{ID: "3", Source: "gupy", Seniority: []string{"mid", "senior"}},
		{ID: "4", Source: "lever", Seniority: []string{}},
	}
	ids := func(res discoveryResult) []string {
		var out []string
		for _, j := range res.Jobs {
			out = append(out, j.ID)
		}
		return out
	}

	pleno := discoveryFilter{Seniority: discovery.SeniorityPrefs{Levels: []string{"mid"}, IncludeUnknown: true}}
	res := filterDiscovered(jobs, pleno)
	names(t, ids(res), "1", "3", "4")

	pleno.Seniority.IncludeUnknown = false
	names(t, ids(filterDiscovered(jobs, pleno)), "1", "3")

	// A contagem de níveis ignora o próprio filtro de nível e lista todos, na ordem.
	counts := map[string]int{}
	var order []string
	for _, f := range res.Facets.Levels {
		counts[f.Name] = f.Count
		order = append(order, f.Name)
	}
	names(t, order, "intern", "junior", "mid", "senior", "expert", "unknown")
	if counts["mid"] != 2 || counts["senior"] != 2 || counts["unknown"] != 1 {
		t.Errorf("contagem de níveis: %v", counts)
	}
	// A de plataformas respeita o nível: sem vaga sem nível, a lever some.
	for _, f := range filterDiscovered(jobs, pleno).Facets.Sources {
		if f.Name == "lever" {
			t.Errorf("lever não tem vaga pleno: %+v", f)
		}
	}
}

func TestCleanPreferences_Nivel(t *testing.T) {
	got, err := cleanPreferences(DiscoveryPreferences{Seniorities: []string{"senior", "mid", "senior"}})
	if err != nil {
		t.Fatal(err)
	}
	names(t, got.Seniorities, "senior", "mid")
	if got.SeniorityUnknown == nil || !*got.SeniorityUnknown {
		t.Error("sem seniority_unknown no corpo, as vagas sem nível continuam aparecendo")
	}
	no := false
	if got, _ := cleanPreferences(DiscoveryPreferences{SeniorityUnknown: &no}); *got.SeniorityUnknown {
		t.Error("seniority_unknown=false deveria ser respeitado")
	}
	if _, err := cleanPreferences(DiscoveryPreferences{Seniorities: []string{"pleno"}}); err == nil {
		t.Error("nível fora da lista deveria dar erro")
	}
}

func TestOtherStack(t *testing.T) {
	// techs vem da varredura, com as regras de contexto.
	idx := discovery.IndexJob("Engenheiro(a) de Software", "linguagens de programação (C#, Java, Go ou Rust); noções de Docker")
	j := DiscoveredJob{
		MatchedSkills: []string{"Go (Golang)"},
		MatchedTechs:  []string{},
		techs:         idx.Techs,
	}
	names(t, otherStack(j), "Java", "C#", "Rust", "Docker")
}

func TestRankDiscovered(t *testing.T) {
	fit := func(q db.JobQuality, strong, partial, pct int) jobfit.Fit {
		items := make([]jobfit.Item, strong+partial+1)
		return jobfit.Fit{Quality: q, Strong: strong, Partial: partial, CoveragePct: pct, Items: items}
	}
	jobs := []DiscoveredJob{
		{ID: "so-texto"},
		{ID: "baixa", fit: fit(db.JobQualityLow, 0, 1, 20)},
		{ID: "so-go", fit: fit(db.JobQualityHigh, 1, 0, 100)},
		{ID: "media", fit: fit(db.JobQualityMid, 3, 0, 60)},
		{ID: "go-pg-docker", fit: fit(db.JobQualityHigh, 3, 0, 75)},
	}
	rankDiscovered(jobs)
	var got []string
	for _, j := range jobs {
		got = append(got, j.ID)
	}
	want := []string{"go-pg-docker", "so-go", "media", "baixa", "so-texto"}
	if !slices.Equal(got, want) {
		t.Errorf("ordem = %v, want %v", got, want)
	}
}

func TestIntersect(t *testing.T) {
	if got := intersect([]string{"Go", "Rust", "Java"}, []string{"Java", "Go"}); !slices.Equal(got, []string{"Go", "Java"}) {
		t.Errorf("intersect = %v", got)
	}
	if got := intersect(nil, []string{"Go"}); got == nil || len(got) != 0 {
		t.Errorf("intersect vazio = %#v, want []string{}", got)
	}
}
