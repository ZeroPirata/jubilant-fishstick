package discovery

import (
	"slices"
	"strings"
)

// Role é a função da vaga (back-end, front-end, dados...), lida do título. O Descobrir
// usa para ordenar e esconder vagas de outra função: "Go" sozinho traz vaga de dados,
// DevOps e back-end misturadas; "Back-end + Go" separa.
type Role struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	// Markers: como aparece no título, em minúsculas e sem acento, palavra inteira.
	Markers []string `json:"-"`
	// LinkedIn: termos para a busca booleana do LinkedIn (com acento, como se escreve).
	LinkedIn []string `json:"-"`
	// Tech: é função de desenvolvimento (entra no Alvo sugerido pelo perfil). Produto,
	// design e suporte existem só para reconhecer vaga de outra função.
	Tech bool `json:"tech"`
}

// Roles: o catálogo, na ordem da tela. "api" ficou de fora dos marcadores de back-end
// ("Analista de API Management" não é back-end), e "data" sozinho conta como dados
// ("Data Platform Engineer").
var Roles = []Role{
	{Key: "backend", Label: "Back-end", Tech: true,
		Markers:  []string{"backend", "back-end", "back end", "server side", "server-side"},
		LinkedIn: []string{"backend", "back-end", "back end"}},
	{Key: "frontend", Label: "Front-end", Tech: true,
		Markers:  []string{"frontend", "front-end", "front end"},
		LinkedIn: []string{"frontend", "front-end", "front end"}},
	{Key: "fullstack", Label: "Full stack", Tech: true,
		Markers:  []string{"fullstack", "full stack", "full-stack"},
		LinkedIn: []string{"fullstack", "full stack", "full-stack"}},
	{Key: "mobile", Label: "Mobile", Tech: true,
		Markers:  []string{"mobile", "android", "ios", "flutter", "react native"},
		LinkedIn: []string{"mobile", "android", "iOS", "flutter"}},
	{Key: "data", Label: "Dados", Tech: true,
		Markers: []string{"dados", "data", "bi", "business intelligence", "machine learning", "ml", "analytics",
			"cientista", "scientist", "etl", "big data"},
		LinkedIn: []string{"dados", "data engineer", "data scientist", "data analyst", "machine learning"}},
	{Key: "devops", Label: "DevOps e cloud", Tech: true,
		Markers: []string{"devops", "dev ops", "sre", "site reliability", "infraestrutura", "infrastructure", "cloud",
			"platform engineer", "engenheiro de plataforma", "devsecops"},
		LinkedIn: []string{"devops", "SRE", "cloud", "infraestrutura"}},
	{Key: "qa", Label: "QA e testes", Tech: true,
		Markers: []string{"qa", "quality assurance", "teste", "testes", "tester", "test", "sdet", "automacao de testes",
			"qualidade de software"},
		LinkedIn: []string{"QA", "testes", "quality assurance"}},
	{Key: "security", Label: "Segurança", Tech: true,
		Markers:  []string{"seguranca", "security", "cybersecurity", "appsec", "pentest", "pentester"},
		LinkedIn: []string{"segurança", "security", "cybersecurity"}},
	{Key: "product", Label: "Produto",
		Markers: []string{"product owner", "product manager", "gerente de produto", "analista de produto", "product analyst"}},
	{Key: "design", Label: "Design",
		Markers: []string{"designer", "ux", "ui/ux", "ux/ui", "ux designer", "ui designer"}},
	{Key: "support", Label: "Suporte",
		Markers: []string{"suporte", "support", "helpdesk", "help desk", "service desk"}},
}

// roleNeighbors: funções próximas, que não somem da lista de quem procura a outra.
// Quem procura back-end ainda vê full stack (e vice-versa), um pouco abaixo.
var roleNeighbors = map[string][]string{
	"backend":   {"fullstack"},
	"frontend":  {"fullstack"},
	"fullstack": {"backend", "frontend"},
	"devops":    {"backend"},
}

var roleByKey = func() map[string]Role {
	m := make(map[string]Role, len(Roles))
	for _, r := range Roles {
		m[r.Key] = r
	}
	return m
}()

// RoleByKey acha a função pela chave.
func RoleByKey(key string) (Role, bool) {
	r, ok := roleByKey[key]
	return r, ok
}

// ClassifyRoles acha as funções citadas no título da vaga, na ordem de Roles. A
// descrição não entra: "trabalhar com o time de front-end" não faz da vaga front-end.
func ClassifyRoles(title string) []string {
	t := foldAccents(strings.ToLower(title))
	out := []string{}
	for _, r := range Roles {
		if hasAny(t, r.Markers) {
			out = append(out, r.Key)
		}
	}
	// "Full stack" já diz back e front: não conta também como cada um.
	if slices.Contains(out, "fullstack") {
		out = slices.DeleteFunc(out, func(k string) bool { return k == "backend" || k == "frontend" })
	}
	return out
}

// Fit da função da vaga com as funções procuradas.
const (
	RoleMatch    = "match"    // a vaga é de uma função procurada
	RoleNeighbor = "neighbor" // função próxima (full stack para quem procura back-end)
	RoleUnknown  = "unknown"  // o título não diz a função ("Desenvolvedor Go")
	RoleOther    = "other"    // outra função: some da lista
)

// RoleFit compara as funções da vaga com as procuradas. Sem função procurada, tudo é
// RoleUnknown (não ordena nem esconde).
func RoleFit(jobRoles, wanted []string) string {
	if len(wanted) == 0 || len(jobRoles) == 0 {
		return RoleUnknown
	}
	for _, r := range jobRoles {
		if slices.Contains(wanted, r) {
			return RoleMatch
		}
	}
	for _, w := range wanted {
		for _, r := range jobRoles {
			if slices.Contains(roleNeighbors[w], r) {
				return RoleNeighbor
			}
		}
	}
	return RoleOther
}
