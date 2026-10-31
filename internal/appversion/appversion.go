// Package appversion lê a versão do app (version.json na raiz, embutido no binário).
//
// Regra do deploy (scripts/check_version.sh): cada deploy precisa de uma versão maior
// que a que está no ar e de uma data igual ou posterior à dela.
package appversion

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	hacktontreino "hackton-treino"
)

// Info é o conteúdo do version.json.
type Info struct {
	Version string `json:"version"`
	// Date: dia da atualização, AAAA-MM-DD.
	Date string `json:"date"`
}

var semver = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// Parse lê e valida o version.json: versão X.Y.Z e data AAAA-MM-DD válida.
func Parse(data []byte) (Info, error) {
	var info Info
	if err := json.Unmarshal(data, &info); err != nil {
		return Info{}, fmt.Errorf("version.json inválido: %w", err)
	}
	if !semver.MatchString(info.Version) {
		return Info{}, fmt.Errorf("version.json: versão %q fora do formato X.Y.Z", info.Version)
	}
	if _, err := time.Parse(time.DateOnly, info.Date); err != nil {
		return Info{}, fmt.Errorf("version.json: data %q fora do formato AAAA-MM-DD", info.Date)
	}
	return info, nil
}

var current = func() Info {
	info, err := Parse(hacktontreino.VersionJSON)
	if err != nil {
		// O teste do pacote barra um version.json inválido antes do deploy.
		return Info{Version: "desconhecida"}
	}
	return info
}()

// Current é a versão deste binário.
func Current() Info { return current }

// Compare devolve -1, 0 ou 1 comparando duas versões X.Y.Z número a número.
func Compare(a, b string) int {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < 3; i++ {
		na, _ := strconv.Atoi(pa[i])
		nb, _ := strconv.Atoi(pb[i])
		switch {
		case na < nb:
			return -1
		case na > nb:
			return 1
		}
	}
	return 0
}

// CheckRelease aplica a regra do deploy: next precisa ter versão maior que deployed e
// data igual ou posterior.
func CheckRelease(deployed, next Info) error {
	if Compare(next.Version, deployed.Version) <= 0 {
		return fmt.Errorf("versão %s não é maior que a do ar (%s): aumente a versão no version.json", next.Version, deployed.Version)
	}
	if next.Date < deployed.Date {
		return fmt.Errorf("data %s é anterior à da versão no ar (%s)", next.Date, deployed.Date)
	}
	return nil
}
