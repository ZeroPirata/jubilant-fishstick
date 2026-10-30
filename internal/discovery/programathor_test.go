package discovery

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

const programathorPage = `<html><body>
<div class="cell-list "><a href="/jobs/33685-desenvolvedor-back-end-senior">
 <h3 class="text-24">Desenvolvedor Back-End Senior (AI Engineer)</h3>
 <div class='cell-list-content-icon'><span><i class='fa fa-briefcase'></i>LOLDESIGN</span><span><i class='fas fa-map-marker-alt'></i>Remoto</span><span><i class='far fa-chart-bar'></i>Sênior</span><span><i class='far fa-file-alt'></i>PJ</span></div>
 <div><span class='tag-list background-gray'>Python</span><span class='tag-list background-gray'>Kafka</span></div>
</a></div>
<div class="cell-list "><a href="/jobs/33700-dev-java">
 <h3 class="text-24">Dev Java</h3>
 <div class='cell-list-content-icon'><span><i class='fa fa-briefcase'></i>Acme</span><span><i class='fas fa-map-marker-alt'></i>Vinhedo  (Presencial)</span></div>
</a></div>
<div class="cell-list "><a href="/jobs/30000-antiga">
 <h3><span class="color-red">Vencida</span> Dev PHP</h3>
</a></div>
</body></html>`

func TestParseProgramaThor(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(programathorPage))
	if err != nil {
		t.Fatal(err)
	}
	got, open := parseProgramaThor(doc)
	if open != 2 || len(got) != 2 {
		t.Fatalf("a vencida deveria sair: %d %+v", open, got)
	}
	a := got[0]
	if a.ExternalID != "33685" || a.Company != "LOLDESIGN" || a.WorkplaceType != "remote" || a.Location != "" ||
		a.URL != "https://programathor.com.br/jobs/33685-desenvolvedor-back-end-senior" {
		t.Errorf("primeira vaga: %+v", a)
	}
	if !strings.Contains(a.Description, "Tecnologias: Python, Kafka") || !strings.Contains(a.Description, "Nível: Sênior") {
		t.Errorf("descrição: %q", a.Description)
	}
	if b := got[1]; b.Location != "Vinhedo" || b.WorkplaceType != "onsite" {
		t.Errorf("local e modo da segunda: %+v", b)
	}
}
