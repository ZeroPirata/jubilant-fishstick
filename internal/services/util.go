package services

import "strings"

func stripMarkdownCode(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		s = s[strings.Index(s, "\n")+1:]
	}
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

func sanitizeJSONLiterals(s string) string {
	var buf strings.Builder
	buf.Grow(len(s))
	inString := false
	escaped := false
	for _, c := range s {
		if escaped {
			buf.WriteRune(c)
			escaped = false
			continue
		}
		if c == '\\' {
			buf.WriteRune(c)
			escaped = true
			continue
		}
		if c == '"' {
			inString = !inString
			buf.WriteRune(c)
			continue
		}
		if inString {
			switch c {
			case '\n':
				buf.WriteString(`\n`)
			case '\r':
				buf.WriteString(`\r`)
			case '\t':
				buf.WriteString(`\t`)
			default:
				buf.WriteRune(c)
			}
			continue
		}
		buf.WriteRune(c)
	}
	return buf.String()
}
