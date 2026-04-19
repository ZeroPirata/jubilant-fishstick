package handler

import (
	"context"
	"encoding/json"
	"hackton-treino/internal/middleware"
	"hackton-treino/internal/repository"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
)

type PaginationParams struct {
	Cursor int32
	Size   int32
	Search *string
}

// GenericList busca uma lista paginada de recursos.
// buildParams recebe userID + cursor + size e retorna os parâmetros da query.
// exec executa a query e retorna as rows.
// mapRow converte cada row do banco para o tipo de resposta Res.
// totalCount extrai o total de registros da primeira row (padrão COUNT(*) OVER() do sqlc).
func GenericList[Params any, Row any, Res any](
	buildParams func(userID pgtype.UUID, query PaginationParams) Params,
	exec func(ctx context.Context, params Params) ([]Row, *repository.RepositoryError),
	mapRow func(Row) Res,
	totalCount func(Row) int32,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cursor, size := getPaginationParams(r)

		userID, ok := middleware.GetUserID(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, ErrNotAuthorized.Error())
			return
		}

		rows, errR := exec(r.Context(), buildParams(userID, PaginationParams{
			Cursor: cursor,
			Size:   size,
			Search: getSearchParam(r),
		}))
		if errR != nil {
			writeRepositoryError(w, errR)
			return
		}

		if len(rows) == 0 {
			writeList(w, http.StatusOK, []Res{}, size, cursor, 0)
			return
		}

		response := make([]Res, len(rows))
		for i, row := range rows {
			response[i] = mapRow(row)
		}

		writeList(w, http.StatusOK, response, size, cursor, totalCount(rows[0]))
	}
}

// GenericOne busca um único recurso por parâmetros extraídos da request.
// buildParams recebe o userID e a request (para extrair path values, query params, etc).
// exec executa a query e retorna a row.
// mapRow converte a row do banco para o tipo de resposta Res.
func GenericOne[Params any, Row any, Res any](
	buildParams func(userID pgtype.UUID, r *http.Request) (Params, *repository.RepositoryError),
	exec func(ctx context.Context, params Params) (Row, *repository.RepositoryError),
	mapRow func(Row) Res,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.GetUserID(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, ErrNotAuthorized.Error())
			return
		}

		params, errR := buildParams(userID, r)
		if errR != nil {
			writeRepositoryError(w, errR)
			return
		}

		row, errR := exec(r.Context(), params)
		if errR != nil {
			writeRepositoryError(w, errR)
			return
		}

		writeJSON(w, http.StatusOK, mapRow(row))
	}
}

// GenericCreate decodifica o body em T, valida e executa a operação de criação.
// validate pode ser nil para pular validação.
func GenericCreate[T any](
	exec func(ctx context.Context, userID pgtype.UUID, body T) *repository.RepositoryError,
	validate func(T) error,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.GetUserID(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, ErrNotAuthorized.Error())
			return
		}

		var body T
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, ErrInvalidRequestBody.Error())
			return
		}

		if validate != nil {
			if err := validate(body); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
		}

		if errR := exec(r.Context(), userID, body); errR != nil {
			writeRepositoryError(w, errR)
			return
		}

		w.WriteHeader(http.StatusCreated)
	}
}

// GenericDelete extrai o userID do contexto e executa a operação de deleção.
// buildParams recebe o userID e a request para montar os parâmetros da query.
func GenericDelete[T any](
	buildParams func(userID pgtype.UUID, r *http.Request) (T, *repository.RepositoryError),
	exec func(ctx context.Context, params T) *repository.RepositoryError,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.GetUserID(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, ErrNotAuthorized.Error())
			return
		}

		params, errR := buildParams(userID, r)
		if errR != nil {
			writeRepositoryError(w, errR)
			return
		}

		if errR = exec(r.Context(), params); errR != nil {
			writeRepositoryError(w, errR)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// GenericUpdate decodifica o body em T, valida e executa a operação de atualização.
// buildParams combina o userID, a request e o body decodificado para montar os parâmetros.
func GenericUpdate[Body any, Params any](
	buildParams func(userID pgtype.UUID, r *http.Request, body Body) (Params, *repository.RepositoryError),
	exec func(ctx context.Context, params Params) *repository.RepositoryError,
	validate func(Body) error,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.GetUserID(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, ErrNotAuthorized.Error())
			return
		}

		var body Body
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, ErrInvalidRequestBody.Error())
			return
		}

		if validate != nil {
			if err := validate(body); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
		}

		params, errR := buildParams(userID, r, body)
		if errR != nil {
			writeRepositoryError(w, errR)
			return
		}

		if errR = exec(r.Context(), params); errR != nil {
			writeRepositoryError(w, errR)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
