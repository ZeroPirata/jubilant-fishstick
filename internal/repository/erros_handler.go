package repository

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5/pgconn"
)

type AppError struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
}

func (e AppError) Error() string {
	return e.Message
}

func HandleDatabaseError(err error) *AppError {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return &AppError{StatusCode: http.StatusNotFound, Message: "resource not found"}
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			return &AppError{StatusCode: http.StatusConflict, Message: "duplicate entry"}
		case "23503": // foreign_key_violation
			return &AppError{StatusCode: http.StatusBadRequest, Message: "foreign key violation"}
		case "23502": // not_null_violation
			return &AppError{StatusCode: http.StatusBadRequest, Message: "required field missing"}
		default:
			return &AppError{StatusCode: http.StatusInternalServerError, Message: "database error"}
		}
	}

	return &AppError{StatusCode: http.StatusInternalServerError, Message: "unexpected error"}
}
