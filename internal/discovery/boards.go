package discovery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	// Pausa entre empresas no mesmo ATS.
	boardDelay = 300 * time.Millisecond
	// Quadros grandes demoram: a Lever às vezes leva mais de 30 s para começar a
	// responder o da Toptal (1,2 MB), que no resto do tempo vem em 2 s.
	boardClientTimeout = 60 * time.Second
)

// Espera antes da segunda tentativa de um pedido que estourou o tempo (variável para
// os testes).
var boardRetryDelay = 5 * time.Second

func newBoardClient() *http.Client {
	return &http.Client{Timeout: boardClientTimeout}
}

// listBoards lê uma empresa por passo. Empresa que falha (saiu do ATS, token errado)
// entra como falha e não interrompe as outras; o erro lista todas as que falharam.
func listBoards(ctx context.Context, boards []Board, step StepFunc, fetch func(context.Context, Board) ([]Listing, error)) ListResult {
	var res ListResult
	var failures []string
	for i, b := range boards {
		if i > 0 && !wait(ctx, boardDelay) {
			res.Failed += len(boards) - i
			failures = append(failures, ctx.Err().Error())
			break
		}
		step(i, len(boards), b.Company)
		listings, err := fetch(ctx, b)
		if err != nil {
			res.Failed++
			failures = append(failures, fmt.Sprintf("%s (%v)", b.Token, err))
			continue
		}
		res.OK++
		res.Listings = append(res.Listings, listings...)
	}
	if len(failures) > 0 {
		res.LastError = strings.Join(failures, "; ")
	}
	step(len(boards), len(boards), "")
	return res
}

// getJSON faz o GET (esperando em caso de limite) e decodifica a resposta em out. Um
// pedido que estoura o tempo é tentado de novo uma vez: a lentidão dos ATS costuma
// ser passageira.
func getJSON(ctx context.Context, client *http.Client, url string, out any) error {
	err := getJSONOnce(ctx, client, url, out)
	if isTimeout(err) && wait(ctx, boardRetryDelay) {
		err = getJSONOnce(ctx, client, url, out)
	}
	return err
}

// isTimeout: o cliente desistiu de esperar (Client.Timeout), não o contexto da busca.
func isTimeout(err error) bool {
	var ne net.Error
	return err != nil && errors.As(err, &ne) && ne.Timeout()
}

func getJSONOnce(ctx context.Context, client *http.Client, url string, out any) error {
	resp, err := getWithRetry(ctx, client, url, http.Header{"Accept": {"application/json"}}, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return errBoardNotFound
	case resp.StatusCode != http.StatusOK:
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("JSON inválido: %w", err)
	}
	return nil
}
