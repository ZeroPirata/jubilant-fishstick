package handler

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"
)

func TestAssetVersioner(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("index.html", `<link rel="stylesheet" href="/static/css/a.css">
<link rel="preload" href="/static/fonts/f.woff2" as="font">
<script src="/static/js/app.js"></script>
<script src="/static/js/sumiu.js"></script>`)
	write("css/a.css", "body{}")
	write("js/app.js", "let a = 1")
	a := &assetVersioner{dir: dir}

	page, err := a.render()
	if err != nil {
		t.Fatal(err)
	}
	got := string(page)
	js := regexp.MustCompile(`/static/js/app\.js\?v=([0-9a-f]{12})"`).FindStringSubmatch(got)
	if js == nil || !regexp.MustCompile(`/static/css/a\.css\?v=[0-9a-f]{12}"`).MatchString(got) {
		t.Fatalf("sem versão nos assets:\n%s", got)
	}
	if !regexp.MustCompile(`/static/fonts/f\.woff2"`).MatchString(got) || !regexp.MustCompile(`/static/js/sumiu\.js"`).MatchString(got) {
		t.Fatalf("fonte e arquivo inexistente devem ficar como estão:\n%s", got)
	}

	// Conteúdo novo, versão nova (mtime adiantado: o teste roda rápido demais).
	write("js/app.js", "let a = 2")
	future := time.Now().Add(time.Minute)
	os.Chtimes(filepath.Join(dir, "js/app.js"), future, future)
	page, _ = a.render()
	js2 := regexp.MustCompile(`/static/js/app\.js\?v=([0-9a-f]{12})"`).FindStringSubmatch(string(page))
	if js2 == nil || js2[1] == js[1] {
		t.Fatalf("versão não mudou com o conteúdo: %v → %v", js, js2)
	}
}

func TestStaticCache(t *testing.T) {
	h := StaticCache(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	cases := map[string]string{
		"js/app.js?v=abc":   "public, max-age=31536000, immutable",
		"js/app.js":         "no-cache",
		"fonts/x.woff2":     "public, max-age=2592000",
		"css/style.css?v=1": "public, max-age=31536000, immutable",
	}
	for url, want := range cases {
		// O handler real recebe o caminho já sem /static/ (StripPrefix).
		r := httptest.NewRequest("GET", "/"+url, nil)
		r.URL.Path = r.URL.Path[1:]
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if got := w.Header().Get("Cache-Control"); got != want {
			t.Errorf("%s: Cache-Control = %q, quero %q", url, got, want)
		}
	}
}
