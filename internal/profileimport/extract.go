// Package profileimport lê o currículo que o usuário envia (PDF, DOCX ou texto, inclusive
// o PDF exportado do LinkedIn), pede à IA os dados estruturados e devolve um rascunho
// para o usuário conferir antes de qualquer coisa ir para o perfil.
package profileimport

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	MaxFileSize = 5 << 20 // 5 MB: currículo com foto/ícones cabe com folga
	maxTextLen  = 40000   // ~10 páginas de texto; o resto não muda o perfil e custa tokens
	minTextLen  = 150     // abaixo disso é PDF escaneado (imagem) ou arquivo vazio
	maxPDFPages = 10
	maxDocxXML  = 20 << 20 // limite do document.xml descompactado (zip bomb)
)

var (
	ErrUnsupported = errors.New("formato não suportado: envie um PDF, DOCX ou TXT")
	ErrNoText      = errors.New("não encontrei texto no arquivo. Se for um PDF escaneado (imagem), exporte o currículo de novo como PDF de texto")
)

// Kind identifica o formato pelo conteúdo (não pela extensão): pdf, docx, txt ou "".
func Kind(data []byte) string {
	switch {
	case bytes.HasPrefix(data, []byte("%PDF-")):
		return "pdf"
	case bytes.HasPrefix(data, []byte("PK\x03\x04")):
		return "docx"
	case utf8.Valid(data) && !bytes.ContainsRune(data, 0):
		return "txt"
	}
	return ""
}

// ExtractText identifica o formato pelo conteúdo (não pela extensão) e devolve o texto limpo.
func ExtractText(ctx context.Context, data []byte) (string, error) {
	var (
		text string
		err  error
	)
	switch Kind(data) {
	case "pdf":
		text, err = pdfText(ctx, data)
	case "docx":
		text, err = docxText(data)
	case "txt":
		text = string(data)
	default:
		return "", ErrUnsupported
	}
	if err != nil {
		return "", err
	}

	text = cleanText(text)
	if utf8.RuneCountInString(text) < minTextLen {
		return "", ErrNoText
	}
	if len(text) > maxTextLen {
		text = text[:maxTextLen]
		for !utf8.ValidString(text) {
			text = text[:len(text)-1]
		}
	}
	return text, nil
}

// pdfText usa o pdftotext (poppler): ordem de leitura correta inclusive em layouts de
// duas colunas, como o PDF do LinkedIn.
func pdfText(ctx context.Context, data []byte) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "pdftotext", "-enc", "UTF-8", "-q", "-l", fmt.Sprint(maxPDFPages), "-", "-")
	cmd.Stdin = bytes.NewReader(data)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", fmt.Errorf("pdftotext não instalado no servidor (pacote poppler-utils): %w", err)
		}
		return "", fmt.Errorf("não consegui ler o PDF (arquivo protegido ou corrompido?): %v %s", err, strings.TrimSpace(stderr.String()))
	}
	return out.String(), nil
}

// docxText lê o word/document.xml: cada parágrafo vira uma linha.
func docxText(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", ErrUnsupported
	}
	for _, f := range zr.File {
		if f.Name != "word/document.xml" {
			continue
		}
		if f.UncompressedSize64 > maxDocxXML {
			return "", errors.New("documento grande demais")
		}
		rc, err := f.Open()
		if err != nil {
			return "", fmt.Errorf("não consegui abrir o DOCX: %w", err)
		}
		defer rc.Close()
		return docxXMLText(io.LimitReader(rc, maxDocxXML))
	}
	// Um ZIP sem document.xml não é DOCX (ex.: .odt, .xlsx).
	return "", ErrUnsupported
}

func docxXMLText(r io.Reader) (string, error) {
	var b strings.Builder
	dec := xml.NewDecoder(r)
	inText := false
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", fmt.Errorf("DOCX inválido: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "t":
				inText = true
			case "tab":
				b.WriteByte('\t')
			case "br":
				b.WriteByte('\n')
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "t":
				inText = false
			case "p":
				b.WriteByte('\n')
			}
		case xml.CharData:
			if inText {
				b.Write(t)
			}
		}
	}
	return b.String(), nil
}

var (
	reInvisible  = regexp.MustCompile(`[\x{200b}\x{200c}\x{200d}\x{2060}\x{feff}\x{00ad}]`)
	rePageMarker = regexp.MustCompile(`(?im)^\s*(page|página|pagina)\s+\d+\s+(of|de)\s+\d+\s*$`)
	reSpaces     = regexp.MustCompile(`[ \t\x{00a0}]+`)
	reBlankLines = regexp.MustCompile(`\n{3,}`)
)

// cleanText tira o que só atrapalha a IA: caracteres invisíveis, quebras de página,
// rodapés "Page 1 of 3" do LinkedIn e espaços repetidos.
func cleanText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\f", "\n")
	s = reInvisible.ReplaceAllString(s, "")
	s = rePageMarker.ReplaceAllString(s, "")
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSpace(reSpaces.ReplaceAllString(l, " "))
	}
	s = strings.Join(lines, "\n")
	return strings.TrimSpace(reBlankLines.ReplaceAllString(s, "\n\n"))
}
