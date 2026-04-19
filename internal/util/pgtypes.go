package util

import (
	"hackton-treino/internal/repository"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func ParsePgDate(s string) pgtype.Date {
	if s == "" {
		return pgtype.Date{Valid: false}
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return pgtype.Date{Valid: false}
	}
	return pgtype.Date{Time: t, Valid: true}
}

func ParseUUID(id string) (pgtype.UUID, *repository.RepositoryError) {
	var u pgtype.UUID
	if err := u.Scan(id); err != nil {
		return pgtype.UUID{}, &repository.RepositoryError{StatusCode: http.StatusBadRequest, Message: "invalid id format"}
	}
	return u, nil
}

func ConvertToPgText(text string) pgtype.Text {
	if text == "" {
		return pgtype.Text{
			Valid: false,
		}
	}

	return pgtype.Text{
		String: text,
		Valid:  true,
	}
}

func ConvertToPgTextPtr(text *string) pgtype.Text {
	if text == nil || *text == "" {
		return pgtype.Text{
			Valid: false,
		}
	}

	return pgtype.Text{
		String: *text,
		Valid:  true,
	}
}

func ConvertToPgTextArray(texts []string) []string {
	if len(texts) == 0 {
		return nil
	}

	var pgArray []string
	pgArray = append(pgArray, texts...)

	return pgArray
}
