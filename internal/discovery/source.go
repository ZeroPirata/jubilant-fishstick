// Package discovery busca vagas em sites de emprego (a aba "Descobrir"). Cada site é
// uma Source; o Service decide quando buscar e grava o resultado em discovered_jobs.
package discovery

import (
	"context"
	"time"
)

// Listing é uma vaga como veio da fonte. Description precisa vir preenchida: é ela
// que permite ordenar as vagas pelas skills do usuário sem abrir a página de cada uma.
type Listing struct {
	Source        string
	ExternalID    string
	Title         string
	Company       string
	Location      string
	WorkplaceType string // remote | hybrid | onsite, quando a fonte informa
	// International: empresa de fora, remota, aceitando candidatos no Brasil ("Exterior").
	International bool
	URL           string
	Description   string
	// Seniority: níveis da vaga (SeniorityLevels); vazio quando não dá para saber.
	Seniority   []string
	PublishedAt time.Time
	ExpiresAt   time.Time
}

const (
	KindSearch = "search"
	KindList   = "list"
)

type Source interface {
	Name() string
}

// Searcher é a fonte que só sabe buscar por palavra-chave (Gupy): cada termo é um passo.
type Searcher interface {
	Source
	// Search devolve uma página (começando em 1) de vagas para a palavra-chave e se
	// existem mais páginas.
	Search(ctx context.Context, keyword string, page int) ([]Listing, bool, error)
}

// Lister é a fonte que entrega a lista inteira de vagas de uma vez (API sem busca, ou
// com lista pequena o bastante para baixar toda). Os termos não importam: tudo vai
// para o catálogo e o casamento com as skills acontece na hora de listar.
type Lister interface {
	Source
	// List baixa todas as vagas. boards são as empresas configuradas no painel (só as
	// fontes de ATS usam). step informa o andamento: passos feitos, total (0 se ainda
	// não se sabe) e o que está sendo lido agora.
	List(ctx context.Context, boards []Board, step StepFunc) ListResult
}

type StepFunc func(done, total int, current string)

// ListResult traz as vagas lidas mesmo quando parte falhou (uma empresa que saiu do
// ATS não pode derrubar as outras). OK/Failed contam listas ou empresas.
type ListResult struct {
	Listings  []Listing
	OK        int
	Failed    int
	LastError string
}

// BoardSource marca as fontes que leem quadros de vagas de empresas (Greenhouse, Lever):
// o painel mostra a lista de empresas delas.
type BoardSource interface {
	Lister
	UsesBoards()
}

// Board é uma empresa num ATS: o token da URL pública do quadro e o nome a exibir.
type Board struct {
	Token   string `json:"token"`
	Company string `json:"company"`
}

// SourceKind diz se a fonte busca por termo ou baixa a lista inteira.
func SourceKind(src Source) string {
	if _, ok := src.(Lister); ok {
		return KindList
	}
	return KindSearch
}

// UsesBoards diz se a fonte lê as empresas configuradas no painel.
func UsesBoards(src Source) bool {
	_, ok := src.(BoardSource)
	return ok
}

// wait pausa entre pedidos ao mesmo site; false se o contexto acabou.
func wait(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}
