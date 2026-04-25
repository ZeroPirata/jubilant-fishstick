package handler

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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

// OutputBaseDir returns the filesystem root where PDFs are stored.
// Defaults to "output" (relative to working dir) when OUTPUT_DIR is unset.
func OutputBaseDir() string {
	if d := os.Getenv("OUTPUT_DIR"); d != "" {
		return d
	}
	return "output"
}

// buildOutputDir returns (fsDir, urlPath, err).
//   - fsDir   is the directory Python writes PDFs into.
//   - urlPath is the prefix stored in DB and used in download URLs ("/output/…").
//
// resumeID (UUID) is the last segment, guaranteeing uniqueness across users,
// jobs, and multiple generations of the same resume on the same day.
func buildOutputDir(userID, resumeID, empresa, titulo string) (fsDir, urlPath string, err error) {
	if empresa == "" {
		empresa = "empresa"
	}
	if titulo == "" {
		titulo = "vaga"
	}
	subPath := filepath.Join(userID, fmt.Sprintf("%s_%s_%s", normalizarNome(empresa), normalizarNome(titulo), resumeID))
	fsDir = filepath.Join(OutputBaseDir(), subPath)
	urlPath = filepath.Join("output", subPath)
	if err = os.MkdirAll(fsDir, 0o755); err != nil {
		err = fmt.Errorf("criar diretório output: %w", err)
	}
	return
}
