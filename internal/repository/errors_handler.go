package repository

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"
)

type RepositoryError struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
}

func (e RepositoryError) Error() string {
	return e.Message
}

func HandleDatabaseError(err error) *RepositoryError {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return &RepositoryError{StatusCode: http.StatusNotFound, Message: "resource not found"}
	}

	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
		zap.L().Error("database error", zap.String("code", pgErr.Code), zap.String("message", pgErr.Message))
		switch pgErr.Code {
		case "23505": // unique_violation
			return &RepositoryError{StatusCode: http.StatusConflict, Message: "duplicate entry"}
		case "23503": // foreign_key_violation
			return &RepositoryError{StatusCode: http.StatusBadRequest, Message: "foreign key violation"}
		case "23502": // not_null_violation
			return &RepositoryError{StatusCode: http.StatusBadRequest, Message: "required field missing"}
		case "42P01": // undefined_table
			return &RepositoryError{StatusCode: http.StatusInternalServerError, Message: "database table not found"}
		default:
			return &RepositoryError{StatusCode: http.StatusInternalServerError, Message: "internal server error"}
		}
	}

	return &RepositoryError{StatusCode: http.StatusInternalServerError, Message: "unexpected error"}
}
