// Package hacktontreino só existe para embutir o version.json da raiz no binário
// (go:embed não lê arquivos acima do pacote). Quem usa é internal/appversion.
package hacktontreino

import _ "embed"

//go:embed version.json
var VersionJSON []byte
