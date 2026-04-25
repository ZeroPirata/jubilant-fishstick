package util

import (
	"net/http"
	"strings"
	"time"
)

// ClientIP returns the real client IP, preferring Cloudflare's header.
func ClientIP(r *http.Request) string {
	if ip := r.Header.Get("CF-Connecting-IP"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	addr := r.RemoteAddr
	if i := strings.LastIndex(addr, ":"); i != -1 {
		return addr[:i]
	}
	return addr
}

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

func Normalize(text string) string {
	return strings.ToLower(text)
}

func NormalizeStack(entries []string, aliases map[string]string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(entries))

	add := func(s string) {
		s = strings.TrimSpace(strings.ToLower(s))
		if s == "" {
			return
		}
		if canonical, ok := aliases[s]; ok {
			s = canonical
		}
		if _, exists := seen[s]; !exists {
			seen[s] = struct{}{}
			result = append(result, s)
		}
	}

	for _, entry := range entries {
		add(entry)
	}

	return result
}

func PgTextoToNullString(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

func PgTimeToPtr(t time.Time) *time.Time {
	if t.IsZero() || t.Year() <= 1 {
		return nil
	}
	return &t
}
