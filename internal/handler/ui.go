package handler

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// O index.html sai com ?v=<hash do conteúdo> em cada script e folha de estilo. Com a
// versão na URL o navegador guarda o arquivo por um ano (StaticCache) e, quando o
// arquivo muda, o hash muda e ele baixa o novo: deploy sem "Ctrl+F5" e sem revalidar
// os 13 arquivos a cada carregamento.
var uiPage = &assetVersioner{dir: "static"}

// assetRef: referência a /static/... .js ou .css num src/href do index.html.
var assetRef = regexp.MustCompile(`((?:src|href)="/static/)([^"?]+\.(?:js|css))"`)

type assetStamp struct {
	mod  time.Time
	size int64
	hash string
}

// assetVersioner recalcula o hash de um arquivo só quando a data ou o tamanho dele
// mudam: no dev, editar um JS e recarregar a página já traz a versão nova.
type assetVersioner struct {
	dir   string
	mu    sync.Mutex
	files map[string]assetStamp
}

// render devolve o index.html com as versões nas URLs.
func (a *assetVersioner) render() ([]byte, error) {
	page, err := os.ReadFile(filepath.Join(a.dir, "index.html"))
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.files == nil {
		a.files = map[string]assetStamp{}
	}
	return assetRef.ReplaceAllFunc(page, func(m []byte) []byte {
		sub := assetRef.FindSubmatch(m)
		v := a.version(string(sub[2]))
		if v == "" {
			return m
		}
		return append(append(append([]byte{}, sub[1]...), sub[2]...), "?v="+v+`"`...)
	}), nil
}

// version: os primeiros 12 hex do sha256 do arquivo, ou "" se ele não existe (a URL
// fica como está e o 404 aparece normalmente).
func (a *assetVersioner) version(rel string) string {
	if strings.Contains(rel, "..") {
		return ""
	}
	path := filepath.Join(a.dir, filepath.FromSlash(rel))
	st, err := os.Stat(path)
	if err != nil {
		return ""
	}
	if c, ok := a.files[rel]; ok && c.mod.Equal(st.ModTime()) && c.size == st.Size() {
		return c.hash
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	h := hex.EncodeToString(sum[:])[:12]
	a.files[rel] = assetStamp{mod: st.ModTime(), size: st.Size(), hash: h}
	return h
}

func ServeUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache")
	page, err := uiPage.render()
	if err != nil {
		http.ServeFile(w, r, "static/index.html")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(page))
}

// StaticCache define o cache dos arquivos de /static/: com ?v= (URL que muda junto com
// o conteúdo) é guardado por um ano sem revalidar; as fontes, que não mudam, por 30
// dias; o resto revalida a cada uso.
func StaticCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Query().Get("v") != "":
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		case strings.HasPrefix(r.URL.Path, "fonts/"):
			w.Header().Set("Cache-Control", "public, max-age=2592000")
		default:
			w.Header().Set("Cache-Control", "no-cache")
		}
		next.ServeHTTP(w, r)
	})
}
