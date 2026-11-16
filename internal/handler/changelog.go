package handler

import (
	"net/http"

	hacktontreino "hackton-treino"
	"hackton-treino/internal/appversion"
	"hackton-treino/internal/changelog"
)

type changelogResponse struct {
	Current  appversion.Info     `json:"current"`
	Intro    string              `json:"intro"`
	Releases []changelog.Release `json:"releases"`
}

// Changelog (GET /admin/changelog): a aba Novidades do painel admin (admin e master).
// O arquivo vem embutido no binário e não muda enquanto ele roda: monta uma vez.
func Changelog() http.HandlerFunc {
	resp := changelogResponse{Current: appversion.Current()}
	resp.Intro, resp.Releases = changelog.Parse(string(hacktontreino.ChangelogMD))
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, resp)
	}
}
