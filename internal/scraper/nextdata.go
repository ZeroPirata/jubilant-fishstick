package scraper

import (
	"errors"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var ErrNoNextData = errors.New("página sem __NEXT_DATA__ (layout mudou ou acesso bloqueado)")

// NextData devolve o JSON que sites em Next.js embutem na página renderizada no
// servidor. É o mesmo dado que a página usa para se montar, então é bem mais estável
// que depender de classes CSS.
func NextData(doc *goquery.Document) ([]byte, error) {
	raw := strings.TrimSpace(doc.Find("script#__NEXT_DATA__").First().Text())
	if raw == "" {
		return nil, ErrNoNextData
	}
	return []byte(raw), nil
}

// HTMLToText converte um trecho de HTML em texto corrido, um bloco por linha.
func HTMLToText(html string) string {
	if strings.TrimSpace(html) == "" {
		return ""
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return ""
	}
	var b strings.Builder
	doc.Find("p, li, h1, h2, h3, h4").Each(func(_ int, sel *goquery.Selection) {
		// Bloco que contém outros blocos: o texto sai pelos filhos.
		if sel.Find("p, li").Length() > 0 {
			return
		}
		if text := strings.Join(strings.Fields(sel.Text()), " "); text != "" {
			b.WriteString(text)
			b.WriteString("\n")
		}
	})
	if b.Len() == 0 {
		return strings.Join(strings.Fields(doc.Text()), " ")
	}
	return strings.TrimSpace(b.String())
}
