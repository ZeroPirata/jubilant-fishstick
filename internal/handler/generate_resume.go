package handler

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var nonAlphanumRe = regexp.MustCompile(`[^a-z0-9]+`)

func normalizarNome(s string) string {
	s = strings.ToLower(s)
	s = nonAlphanumRe.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

type pythonInput struct {
	Curriculo   string `json:"curriculo"`
	CoverLetter string `json:"cover_letter"`
	OutputDir   string `json:"output_dir"`
}

type pythonOutput struct {
	ResumePath      string `json:"resume_path"`
	CoverLetterPath string `json:"cover_letter_path"`
	Error           string `json:"error,omitempty"`
}

func buildOutputDir(empresa, titulo string) (string, error) {
	if empresa == "" {
		empresa = "empresa"
	}
	if titulo == "" {
		titulo = "vaga"
	}
	data := time.Now().Format("2006-01-02")
	dir := filepath.Join("output", fmt.Sprintf("%s_%s_%s", normalizarNome(empresa), normalizarNome(titulo), data))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("criar diretório output: %w", err)
	}
	return dir, nil
}
