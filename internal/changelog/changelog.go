// Package changelog transforma o CHANGELOG.md (embutido no binário) nas versões da
// aba Novidades do painel admin. Só o pedaço de Markdown que o arquivo usa:
//
//   - "# Título" (ignorado: a tela já tem o seu);
//   - "## 0.4.10 (16/11/2026)" abre uma versão;
//   - parágrafos separados por linha em branco;
//   - listas com "- " (linha seguinte com recuo continua o item);
//   - **negrito**, `código` e [texto](link), só para http(s) e links internos "#/".
//
// Todo texto é escapado antes de virar HTML: o arquivo é nosso, mas a regra é a mesma
// de qualquer texto que vai para a tela. Sem dependência nova: um leitor completo de
// Markdown seria muito mais do que isto precisa.
package changelog

import (
	"fmt"
	"html"
	"regexp"
	"strings"
)

// Release: uma versão do changelog.
type Release struct {
	Version string `json:"version"`
	// Date: DD/MM/AAAA, vazia quando o título não traz data ("Antes da 0.2.0").
	Date string `json:"date"`
	HTML string `json:"html"`
}

var (
	releaseTitle = regexp.MustCompile(`^(.+?)\s*\((\d{2}/\d{2}/\d{4})\)$`)
	code         = regexp.MustCompile("`([^`]+)`")
	bold         = regexp.MustCompile(`\*\*(.+?)\*\*`)
	link         = regexp.MustCompile(`\[([^\]]+)\]\(([^)\s]+)\)`)
)

// Parse devolve o texto antes da primeira versão (em HTML) e as versões, na ordem do
// arquivo (a mais nova primeiro).
func Parse(md string) (intro string, out []Release) {
	var cur *Release
	var body []string
	flush := func() {
		h := Render(strings.Join(body, "\n"))
		if cur == nil {
			intro = h
		} else {
			cur.HTML = h
			out = append(out, *cur)
		}
		body = nil
	}
	for _, line := range strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n") {
		switch {
		case strings.HasPrefix(line, "## "):
			flush()
			title := strings.TrimSpace(line[3:])
			cur = &Release{Version: title}
			if m := releaseTitle.FindStringSubmatch(title); m != nil {
				cur.Version, cur.Date = m[1], m[2]
			}
		case strings.HasPrefix(line, "# "):
			// O título do arquivo: a tela tem o seu.
		default:
			body = append(body, line)
		}
	}
	flush()
	return intro, out
}

// HasVersion: o changelog tem uma entrada para a versão.
func HasVersion(md, version string) bool {
	_, list := Parse(md)
	for _, r := range list {
		if r.Version == version {
			return true
		}
	}
	return false
}

// Render: um bloco (sem "## ") em HTML.
func Render(md string) string {
	var out strings.Builder
	var para, items []string
	closePara := func() {
		if len(para) > 0 {
			fmt.Fprintf(&out, "<p>%s</p>", inline(strings.Join(para, " ")))
			para = nil
		}
	}
	closeList := func() {
		if len(items) > 0 {
			out.WriteString("<ul>")
			for _, it := range items {
				fmt.Fprintf(&out, "<li>%s</li>", inline(it))
			}
			out.WriteString("</ul>")
			items = nil
		}
	}
	for _, raw := range strings.Split(md, "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case line == "":
			closePara()
			closeList()
		case strings.HasPrefix(line, "- "):
			closePara()
			items = append(items, line[2:])
		case len(items) > 0 && strings.HasPrefix(raw, "  "):
			items[len(items)-1] += " " + line
		default:
			closeList()
			para = append(para, line)
		}
	}
	closePara()
	closeList()
	return out.String()
}

// inline: escapa e aplica código, negrito e links.
func inline(text string) string {
	s := html.EscapeString(text)
	s = code.ReplaceAllString(s, "<code>$1</code>")
	s = bold.ReplaceAllString(s, "<strong>$1</strong>")
	return link.ReplaceAllStringFunc(s, func(m string) string {
		p := link.FindStringSubmatch(m)
		label, href := p[1], html.UnescapeString(p[2])
		switch {
		case strings.HasPrefix(href, "#/"):
			return fmt.Sprintf(`<a href="%s">%s</a>`, html.EscapeString(href), label)
		case strings.HasPrefix(href, "https://") || strings.HasPrefix(href, "http://"):
			return fmt.Sprintf(`<a href="%s" target="_blank" rel="noopener noreferrer">%s</a>`, html.EscapeString(href), label)
		}
		return label // esquema desconhecido (javascript:, data:): só o texto
	})
}
