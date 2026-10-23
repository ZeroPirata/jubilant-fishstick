package postgres

import "testing"

func TestQueryLabel(t *testing.T) {
	cases := map[string]string{
		"-- name: QuerySelectAllSkills :many\nSELECT * FROM user_skills":                          "QuerySelectAllSkills",
		"SELECT COUNT(*) FROM error_logs":                                                         "SELECT error_logs",
		"  INSERT INTO ai_calls (purpose) VALUES ($1)":                                            "INSERT ai_calls",
		"UPDATE user_accounts SET is_admin = $1":                                                  "UPDATE user_accounts",
		"DELETE FROM ai_providers WHERE purpose = $1":                                             "DELETE ai_providers",
		"SELECT to_timestamp(floor(extract(epoch FROM created_at) / $2) * $2) AS b FROM ai_calls": "SELECT ai_calls",
		"select 1": "SELECT",
	}
	for sql, want := range cases {
		if got := queryLabel(sql); got != want {
			t.Errorf("queryLabel(%q) = %q, want %q", sql, got, want)
		}
	}
}
