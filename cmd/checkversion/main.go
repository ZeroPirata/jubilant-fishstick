// checkversion barra o deploy quando o version.json não avançou: compara a versão do
// repositório com a que está no ar e aplica appversion.CheckRelease.
//
//	go run ./cmd/checkversion <version.json no ar>
//
// Sem o arquivo do ar (primeiro deploy), só valida o do repositório. Em todo caso, a
// versão precisa ter entrada no CHANGELOG.md.
package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	hacktontreino "hackton-treino"
	"hackton-treino/internal/appversion"
	"hackton-treino/internal/changelog"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "❌", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	next, err := appversion.Parse(hacktontreino.VersionJSON)
	if err != nil {
		return err
	}
	// A aba Novidades do admin mostra o CHANGELOG.md: versão publicada sem entrada lá
	// some da lista.
	if !changelog.HasVersion(string(hacktontreino.ChangelogMD), next.Version) {
		return fmt.Errorf("escreva a entrada \"## %s (DD/MM/AAAA)\" no CHANGELOG.md", next.Version)
	}
	if len(args) != 1 {
		return errors.New("uso: checkversion <version.json no ar>")
	}
	data, err := os.ReadFile(args[0])
	if errors.Is(err, fs.ErrNotExist) {
		fmt.Printf("✅ versão %s (%s); nenhuma versão no ar para comparar\n", next.Version, next.Date)
		return nil
	}
	if err != nil {
		return err
	}
	live, err := appversion.Parse(data)
	if err != nil {
		return fmt.Errorf("versão no ar: %w", err)
	}
	if err := appversion.CheckRelease(live, next); err != nil {
		return err
	}
	fmt.Printf("✅ versão %s (%s) substitui %s (%s)\n", next.Version, next.Date, live.Version, live.Date)
	return nil
}
