package handler

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// A lista de modelos aceitos pela API precisa bater com TEMPLATES em scripts/gerar_pdf.py:
// um modelo só de um lado vira PDF silenciosamente no modelo padrão.
func TestPDFTemplatesBatemComPython(t *testing.T) {
	src, err := os.ReadFile("../../scripts/gerar_pdf.py")
	if err != nil {
		t.Fatal(err)
	}
	block := string(src)
	start := strings.Index(block, "TEMPLATES = {")
	end := strings.Index(block, "DEFAULT_TEMPLATE =")
	if start < 0 || end < start {
		t.Fatal("bloco TEMPLATES não encontrado em gerar_pdf.py")
	}
	var py []string
	for _, m := range regexp.MustCompile(`(?m)^    "(\w+)": \{`).FindAllStringSubmatch(block[start:end], -1) {
		py = append(py, m[1])
	}
	var goKeys []string
	for k := range pdfTemplates {
		goKeys = append(goKeys, k)
	}
	sort.Strings(py)
	sort.Strings(goKeys)
	if strings.Join(py, ",") != strings.Join(goKeys, ",") {
		t.Errorf("modelos divergentes: python=%v go=%v", py, goKeys)
	}
	if !pdfTemplates[defaultPDFTemplate] {
		t.Errorf("modelo padrão %q não está na lista", defaultPDFTemplate)
	}
}
