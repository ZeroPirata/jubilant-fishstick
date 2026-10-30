package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Pausa entre empresas no mesmo ATS.
const boardDelay = 300 * time.Millisecond

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

// getJSON faz o GET (esperando em caso de limite) e decodifica a resposta em out.
func getJSON(ctx context.Context, client *http.Client, url string, out any) error {
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
