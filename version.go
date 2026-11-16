// Package hacktontreino só existe para embutir arquivos da raiz no binário (go:embed
// não lê arquivos acima do pacote): o version.json (internal/appversion) e o
// CHANGELOG.md (internal/changelog, aba Novidades do admin).
package hacktontreino

import _ "embed"

//go:embed version.json
var VersionJSON []byte

// ChangelogMD: o que cada versão mudou, uma seção por versão. O checkversion recusa
// o deploy de uma versão sem entrada aqui.
//
//go:embed CHANGELOG.md
var ChangelogMD []byte
