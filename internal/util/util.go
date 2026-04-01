package util

import (
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
)

func SafeStringSlice(values *[]string) []string {
	if values == nil {
		return nil
	}
	return *values
}

func SafeString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
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

func ConvertToPgTextArray(texts []string) []string {
	if len(texts) == 0 {
		return nil
	}

	var pgArray []string
	pgArray = append(pgArray, texts...)

	return pgArray
}

func Normalize(text string) string {
	return strings.ToLower(text)
}
