package services

import (
	"context"
	"errors"
	"net/url"
	"strconv"
)

// Erros tipados das chamadas à IA: viram o status das métricas por modelo.
var (
	ErrTruncated = errors.New("resposta cortada pelo limite de tokens")
	ErrEmpty     = errors.New("resposta vazia")
	ErrParse     = errors.New("resposta não é o JSON esperado")
	ErrNoContent = errors.New("resposta sem conteúdo no schema esperado")
)

// ErrHTTPStatus é uma resposta HTTP de erro do provider (exceto 429, que é ErrRateLimit).
type ErrHTTPStatus struct {
	Code int
	Body string
}

func (e *ErrHTTPStatus) Error() string {
	return "AiService: status " + strconv.Itoa(e.Code) + ": " + e.Body
}

// Status de uma chamada para as métricas.
const (
	StatusOK         = "ok"
	StatusRateLimit  = "rate_limit"
	StatusTimeout    = "timeout"
	StatusTruncated  = "truncated"
	StatusEmpty      = "empty"
	StatusParseError = "parse_error"
	StatusHTTPError  = "http_error"
	StatusNetworkErr = "network_error"
	StatusOtherError = "error"
)

// ClassifyError converte o erro de uma chamada no status registrado nas métricas.
func ClassifyError(err error) string {
	var rl *ErrRateLimit
	var hs *ErrHTTPStatus
	var ue *url.Error
	switch {
	case err == nil:
		return StatusOK
	case errors.As(err, &rl):
		return StatusRateLimit
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &ue) && ue.Timeout():
		return StatusTimeout
	case errors.Is(err, ErrTruncated):
		return StatusTruncated
	case errors.Is(err, ErrEmpty):
		return StatusEmpty
	case errors.Is(err, ErrParse), errors.Is(err, ErrNoContent):
		return StatusParseError
	case errors.As(err, &hs):
		return StatusHTTPError
	case errors.As(err, &ue):
		return StatusNetworkErr
	default:
		return StatusOtherError
	}
}
