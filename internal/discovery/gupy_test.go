package discovery

import (
	"testing"
	"time"
)

const gupySearchFixture = `{"props":{"pageProps":{"initialJobList":{
	"data":[
		{"id":12717450,"name":" Golang Back-End Developer Sênior ","description":"Serviços em Go.","careerPageName":"Invillia",
		 "publishedDate":"2026-10-13T18:20:55.710Z","applicationDeadline":"2026-12-12T00:00:00.000Z","workplaceType":"remote",
		 "city":"","state":"","jobUrl":"https://invillia.gupy.io/job/abc?jobBoardSource=gupy_portal"},
		{"id":2,"name":"Dev Java","description":"Java","careerPageName":"Acme","publishedDate":"","applicationDeadline":null,
		 "workplaceType":"onsite","city":"Recife","state":"Pernambuco","jobUrl":"https://acme.gupy.io/job/def"},
		{"id":0,"name":"sem id","jobUrl":"https://x.gupy.io/job/1"}
	],
	"pagination":{"total":30,"limit":12,"offset":12}}}}}`

func TestParseGupySearch(t *testing.T) {
	listings, hasMore, err := parseGupySearch([]byte(gupySearchFixture))
	if err != nil {
		t.Fatal(err)
	}
	if len(listings) != 2 {
		t.Fatalf("esperava 2 vagas (a sem id é descartada), veio %d", len(listings))
	}
	if !hasMore {
		t.Error("offset 12 + 3 itens < total 30: deveria ter mais páginas")
	}

	first := listings[0]
	if first.ExternalID != "12717450" || first.Title != "Golang Back-End Developer Sênior" || first.Company != "Invillia" {
		t.Errorf("campos básicos errados: %+v", first)
	}
	if first.Source != "gupy" || first.WorkplaceType != "remote" || first.Location != "" {
		t.Errorf("fonte/modo/local errados: %+v", first)
	}
	if want := time.Date(2026, 12, 12, 0, 0, 0, 0, time.UTC); !first.ExpiresAt.Equal(want) {
		t.Errorf("ExpiresAt = %v, quer %v", first.ExpiresAt, want)
	}

	second := listings[1]
	if second.Location != "Recife, Pernambuco" {
		t.Errorf("Location = %q", second.Location)
	}
	if !second.PublishedAt.IsZero() || !second.ExpiresAt.IsZero() {
		t.Errorf("datas vazias deveriam virar zero: %+v", second)
	}
}

func TestParseGupySearch_UltimaPagina(t *testing.T) {
	raw := `{"props":{"pageProps":{"initialJobList":{"data":[{"id":1,"name":"a","jobUrl":"https://a.gupy.io/job/1"}],"pagination":{"total":13,"limit":12,"offset":12}}}}}`
	_, hasMore, err := parseGupySearch([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if hasMore {
		t.Error("offset 12 + 1 item = total 13: não deveria ter mais páginas")
	}
}
