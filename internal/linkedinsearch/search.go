// Package linkedinsearch monta a URL de busca de vagas do LinkedIn a partir de uma
// busca salva pelo usuário, e lê uma URL colada para preencher a busca.
//
// O JobPipeline não acessa o LinkedIn: só gera o link que o usuário abre no próprio
// navegador, logado, com os filtros que escolheu. Nenhum filtro é ligado por padrão.
//
// Parâmetros da busca de vagas (linkedin.com/jobs/search/):
//
//	keywords   termos da busca
//	geoId      local (código do LinkedIn; só sai de uma URL do próprio LinkedIn)
//	f_TPR      publicadas em: r86400 (24 h), r604800 (semana), r2592000 (mês)
//	f_WT       modelo: 1 presencial, 2 remoto, 3 híbrido
//	f_E        nível: 1 estágio, 2 assistente, 3 júnior, 4 pleno-sênior, 5 diretor, 6 executivo
//	f_JT       tipo: F integral, P meio período, C contrato, T temporário, I estágio, V voluntário, O outro
//	f_AL=true  candidatura simplificada
//	f_EA=true  menos de 10 candidaturas
//	f_JIYN=true na minha rede
//	sortBy     DD (mais recentes) ou R (mais relevantes)
package linkedinsearch

import (
	"errors"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

// Search: os filtros de uma busca. Vazio = sem filtro (o LinkedIn decide).
type Search struct {
	Keywords      string   `json:"keywords"`
	GeoID         string   `json:"geo_id"`
	GeoLabel      string   `json:"geo_label"`
	PostedWithin  string   `json:"posted_within"`
	Workplace     []string `json:"workplace"`
	Experience    []string `json:"experience"`
	JobTypes      []string `json:"job_types"`
	EasyApply     bool     `json:"easy_apply"`
	FewApplicants bool     `json:"few_applicants"`
	InNetwork     bool     `json:"in_network"`
	Sort          string   `json:"sort"`
}

const (
	baseURL     = "https://www.linkedin.com/jobs/search/"
	maxKeywords = 200
	maxGeoLabel = 80
)

// Valores aceitos, na ordem em que vão para a URL.
var (
	postedWithin = []string{"r86400", "r604800", "r2592000"}
	workplace    = []string{"1", "2", "3"}
	experience   = []string{"1", "2", "3", "4", "5", "6"}
	jobTypes     = []string{"F", "P", "C", "T", "I", "V", "O"}
	sorts        = []string{"DD", "R"}
	geoID        = regexp.MustCompile(`^\d{1,20}$`)
)

// Brasil: o geoId do país inteiro (o mesmo dos alertas do LinkedIn).
const GeoBrasil = "106057199"

// Clean valida a busca e põe as listas na ordem fixa, sem repetição. Valor fora da
// lista é erro: a tela só oferece os valores aceitos.
func Clean(s Search) (Search, error) {
	s.Keywords = strings.Join(strings.Fields(s.Keywords), " ")
	s.GeoLabel = strings.Join(strings.Fields(s.GeoLabel), " ")
	s.GeoID = strings.TrimSpace(s.GeoID)
	switch {
	case utf8.RuneCountInString(s.Keywords) > maxKeywords:
		return Search{}, errors.New("palavras-chave longas demais (até 200 caracteres)")
	case utf8.RuneCountInString(s.GeoLabel) > maxGeoLabel:
		return Search{}, errors.New("nome do local longo demais (até 80 caracteres)")
	case s.GeoID != "" && !geoID.MatchString(s.GeoID):
		return Search{}, errors.New("código do local inválido: cole uma URL de busca do LinkedIn")
	case s.PostedWithin != "" && !slices.Contains(postedWithin, s.PostedWithin):
		return Search{}, errors.New("período de publicação inválido")
	case s.Sort != "" && !slices.Contains(sorts, s.Sort):
		return Search{}, errors.New("ordem inválida")
	}
	if s.GeoID == "" {
		s.GeoLabel = ""
	}
	var err error
	if s.Workplace, err = pick(s.Workplace, workplace, "modelo de trabalho"); err != nil {
		return Search{}, err
	}
	if s.Experience, err = pick(s.Experience, experience, "nível"); err != nil {
		return Search{}, err
	}
	if s.JobTypes, err = pick(s.JobTypes, jobTypes, "tipo de vaga"); err != nil {
		return Search{}, err
	}
	return s, nil
}

// pick: os valores escolhidos, na ordem de allowed. Nunca nil (vai para o banco).
func pick(chosen, allowed []string, what string) ([]string, error) {
	out := []string{}
	for _, c := range chosen {
		if !slices.Contains(allowed, c) {
			return nil, errors.New(what + " inválido: " + c)
		}
	}
	for _, a := range allowed {
		if slices.Contains(chosen, a) {
			out = append(out, a)
		}
	}
	return out, nil
}

// URL monta o link da busca. A busca precisa ter passado por Clean.
func URL(s Search) string {
	q := url.Values{}
	if s.Keywords != "" {
		q.Set("keywords", s.Keywords)
	}
	if s.GeoID != "" {
		q.Set("geoId", s.GeoID)
	}
	if s.PostedWithin != "" {
		q.Set("f_TPR", s.PostedWithin)
	}
	if len(s.Workplace) > 0 {
		q.Set("f_WT", strings.Join(s.Workplace, ","))
	}
	if len(s.Experience) > 0 {
		q.Set("f_E", strings.Join(s.Experience, ","))
	}
	if len(s.JobTypes) > 0 {
		q.Set("f_JT", strings.Join(s.JobTypes, ","))
	}
	if s.EasyApply {
		q.Set("f_AL", "true")
	}
	if s.FewApplicants {
		q.Set("f_EA", "true")
	}
	if s.InNetwork {
		q.Set("f_JIYN", "true")
	}
	if s.Sort != "" {
		q.Set("sortBy", s.Sort)
	}
	if len(q) == 0 {
		return baseURL
	}
	// url.Values.Encode ordena as chaves: a mesma busca dá sempre o mesmo link.
	return baseURL + "?" + q.Encode()
}

// FromURL lê uma URL de busca de vagas do LinkedIn (a clássica /jobs/search/ ou a
// nova /jobs/search-results/) e devolve os filtros que ela traz. O que não é filtro
// (vaga aberta, origem, rastreio) fica de fora. O nome do local não vem na URL.
func FromURL(raw string) (Search, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return Search{}, errors.New("cole o endereço completo da busca no LinkedIn (https://www.linkedin.com/jobs/…)")
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	if host != "linkedin.com" && !strings.HasSuffix(host, ".linkedin.com") {
		return Search{}, errors.New("o endereço não é do LinkedIn")
	}
	if !strings.HasPrefix(u.Path, "/jobs/search") {
		return Search{}, errors.New("o endereço não é de uma busca de vagas do LinkedIn")
	}
	q := u.Query()
	list := func(key string) []string {
		var out []string
		for _, v := range q[key] {
			for _, p := range strings.Split(v, ",") {
				if p = strings.TrimSpace(p); p != "" {
					out = append(out, p)
				}
			}
		}
		return out
	}
	// Valor desconhecido (o LinkedIn muda os filtros de vez em quando) é ignorado,
	// em vez de recusar a URL inteira.
	keep := func(vals, allowed []string) []string {
		var out []string
		for _, v := range vals {
			if slices.Contains(allowed, v) {
				out = append(out, v)
			}
		}
		return out
	}
	one := func(key string, allowed []string) string {
		if v := q.Get(key); slices.Contains(allowed, v) {
			return v
		}
		return ""
	}
	s := Search{
		Keywords:      q.Get("keywords"),
		PostedWithin:  one("f_TPR", postedWithin),
		Workplace:     keep(list("f_WT"), workplace),
		Experience:    keep(list("f_E"), experience),
		JobTypes:      keep(list("f_JT"), jobTypes),
		EasyApply:     q.Get("f_AL") == "true",
		FewApplicants: q.Get("f_EA") == "true",
		InNetwork:     q.Get("f_JIYN") == "true",
		Sort:          one("sortBy", sorts),
	}
	if g := q.Get("geoId"); geoID.MatchString(g) {
		s.GeoID = g
		if g == GeoBrasil {
			s.GeoLabel = "Brasil"
		}
	}
	if utf8.RuneCountInString(s.Keywords) > maxKeywords {
		s.Keywords = string([]rune(s.Keywords)[:maxKeywords])
	}
	return Clean(s)
}
