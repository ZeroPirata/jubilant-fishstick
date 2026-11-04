package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"hackton-treino/internal/db"
	"hackton-treino/internal/jobfit"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// ── Seleção automática: o que a IA lê e como a resposta é interpretada ──
// A IA não procura vagas: recebe as mais compatíveis pelo cálculo da stack (o mesmo do
// Descobrir) e julga, como um recrutador do lado do candidato, quais valem a
// candidatura. Ela vê o que o cálculo não vê: senioridade e anos pedidos, idioma,
// requisito eliminatório, tipo de função, local.

// autoSelectBatch: vagas por chamada. Lote pequeno mantém o JSON confiável e a
// resposta dentro do limite de tokens.
const autoSelectBatch = 8

// autoSelectDescMax: quanto da descrição de cada vaga vai para a IA.
const autoSelectDescMax = 2500

const autoSelectSystemPrompt = `Você é um recrutador técnico experiente fazendo a triagem de vagas PARA um candidato (não para a empresa). Para cada vaga, decida se vale a pena o candidato se candidatar, usando apenas o perfil e o texto da vaga.

Critérios, do mais ao menos importante:
1. Requisitos eliminatórios: senioridade ou anos de experiência muito acima (ou muito abaixo) do perfil, idioma exigido que o perfil não mostra, formação ou certificação obrigatória, trabalho presencial/híbrido fora das preferências de local, exigência de cidadania ou visto.
2. Núcleo técnico: o candidato domina as tecnologias principais da vaga (as obrigatórias, não as "desejáveis")? Leve em conta o nível de cada skill.
3. Área e tipo de função: backend × dados × suporte etc.; vaga não técnica, estágio ou freelance para um perfil experiente vale pouco.
4. Chance real de passar na triagem da empresa.

Regras:
- Use só o que está escrito. Não suponha experiência que o perfil não mostra.
- "nota" de 0 a 100: 80+ forte; 60–79 vale tentar; 40–59 arriscada; abaixo de 40 não vale.
- "candidatar": true só com nota 60 ou mais e nenhum requisito eliminatório falhando.
- "motivo": uma frase em português, até 160 caracteres, com o fator decisivo (ex.: "Pede 5+ anos com Kotlin; o perfil tem 2 anos em Go.").
- Avalie todas as vagas da lista, sem pular nenhuma, usando o número entre colchetes.

Responda APENAS com JSON válido, sem texto antes ou depois:
{"avaliacoes":[{"vaga":1,"nota":75,"candidatar":true,"motivo":"..."}]}`

// autoVerdict: o julgamento da IA sobre uma vaga.
type autoVerdict struct {
	Score  int
	Apply  bool
	Reason string
}

// flexInt aceita 7, 7.0 e "7" (modelos variam).
type flexInt int

func (f *flexInt) UnmarshalJSON(b []byte) error {
	s := strings.Trim(strings.TrimSpace(string(b)), `"`)
	if s == "" || s == "null" {
		return errors.New("vazio")
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return err
	}
	*f = flexInt(int(v + 0.5))
	return nil
}

// flexBool aceita true, "true", "sim".
type flexBool bool

func (f *flexBool) UnmarshalJSON(b []byte) error {
	s := strings.ToLower(strings.Trim(strings.TrimSpace(string(b)), `"`))
	*f = flexBool(s == "true" || s == "sim" || s == "yes" || s == "1")
	return nil
}

// parseAutoVerdicts lê a resposta de um lote de n vagas (numeradas 1..n). Erro se
// nenhuma avaliação válida veio: o próximo modelo da cadeia tenta.
func parseAutoVerdicts(content string, n int) (map[int]autoVerdict, error) {
	var out struct {
		Avaliacoes []struct {
			Vaga       flexInt  `json:"vaga"`
			Nota       flexInt  `json:"nota"`
			Candidatar flexBool `json:"candidatar"`
			Motivo     string   `json:"motivo"`
		} `json:"avaliacoes"`
	}
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		return nil, err
	}
	res := map[int]autoVerdict{}
	for _, a := range out.Avaliacoes {
		i := int(a.Vaga)
		if i < 1 || i > n {
			continue
		}
		score := min(max(int(a.Nota), 0), 100)
		reason := strings.TrimSpace(a.Motivo)
		if utf8.RuneCountInString(reason) > 240 {
			reason = string([]rune(reason)[:240]) + "…"
		}
		res[i] = autoVerdict{Score: score, Apply: bool(a.Candidatar), Reason: reason}
	}
	if len(res) == 0 {
		return nil, errors.New("nenhuma avaliação na resposta")
	}
	return res, nil
}

var autoLevelNames = map[string]string{
	"intern": "estágio", "junior": "júnior", "mid": "pleno", "senior": "sênior",
	"expert": "especialista", "manager": "gestão",
}

var autoSkillLevels = map[db.SkillLevel]string{
	db.SkillLevelBasic: "básico", db.SkillLevelIntermediate: "intermediário",
	db.SkillLevelAdvanced: "avançado", db.SkillLevelExpert: "expert",
}

// autoProfileText resume o perfil para a IA: experiências com período e stack,
// tempo total de carreira, skills com nível e as preferências de local e nível. Sem
// nome, e-mail ou contato.
func autoProfileText(exps []db.UserExperience, skills []db.UserSkill, f discoveryFilter, now time.Time) string {
	var b strings.Builder
	b.WriteString("Experiências (mais recente primeiro):\n")
	if len(exps) == 0 {
		b.WriteString("- nenhuma cadastrada\n")
	}
	type span struct{ from, to time.Time }
	var spans []span
	for i, e := range exps {
		if i >= 8 {
			break
		}
		from := e.StartDate.Time
		to := now
		if !e.IsCurrentJob && e.EndDate.Valid {
			to = e.EndDate.Time
		}
		period := "período não informado"
		if e.StartDate.Valid {
			end := "atual"
			if !e.IsCurrentJob && e.EndDate.Valid {
				end = to.Format("01/2006")
			}
			period = fmt.Sprintf("%s – %s, %s", from.Format("01/2006"), end, autoDuration(to.Sub(from)))
			spans = append(spans, span{from, to})
		}
		fmt.Fprintf(&b, "- %s em %s (%s)", e.JobRole, e.CompanyName, period)
		if len(e.TechStack) > 0 {
			fmt.Fprintf(&b, "; stack: %s", strings.Join(e.TechStack, ", "))
		}
		b.WriteString("\n")
	}
	// Tempo total sem contar duas vezes empregos simultâneos.
	sort.Slice(spans, func(i, j int) bool { return spans[i].from.Before(spans[j].from) })
	var total time.Duration
	var cur span
	for i, s := range spans {
		switch {
		case i == 0:
			cur = s
		case s.from.After(cur.to):
			total += cur.to.Sub(cur.from)
			cur = s
		case s.to.After(cur.to):
			cur.to = s.to
		}
	}
	if len(spans) > 0 {
		total += cur.to.Sub(cur.from)
		fmt.Fprintf(&b, "Tempo total de experiência profissional: %s\n", autoDuration(total))
	}

	b.WriteString("\nSkills (nível declarado):\n")
	if len(skills) == 0 {
		b.WriteString("- nenhuma cadastrada\n")
	}
	byLevel := map[db.SkillLevel][]string{}
	for _, s := range skills {
		byLevel[s.ProficiencyLevel] = append(byLevel[s.ProficiencyLevel], s.SkillName)
	}
	for _, lv := range []db.SkillLevel{db.SkillLevelExpert, db.SkillLevelAdvanced, db.SkillLevelIntermediate, db.SkillLevelBasic} {
		if len(byLevel[lv]) > 0 {
			fmt.Fprintf(&b, "- %s: %s\n", autoSkillLevels[lv], strings.Join(byLevel[lv], ", "))
		}
	}

	b.WriteString("\nPreferências:\n")
	places := []string{}
	for _, w := range f.Location.WorkplaceTypes {
		places = append(places, autoPlace(w))
	}
	places = append(places, f.Location.Cities...)
	if len(places) == 0 {
		b.WriteString("- local: qualquer\n")
	} else {
		fmt.Fprintf(&b, "- local: %s\n", strings.Join(places, ", "))
	}
	if len(f.Seniority.Levels) == 0 {
		b.WriteString("- nível: qualquer\n")
	} else {
		lv := make([]string, 0, len(f.Seniority.Levels))
		for _, l := range f.Seniority.Levels {
			if n, ok := autoLevelNames[l]; ok {
				lv = append(lv, n)
			} else {
				lv = append(lv, l)
			}
		}
		fmt.Fprintf(&b, "- nível: %s\n", strings.Join(lv, ", "))
	}
	return b.String()
}

var autoPlaces = map[string]string{"remote": "remoto", "hybrid": "híbrido", "onsite": "presencial"}

func autoPlace(w string) string {
	if p, ok := autoPlaces[w]; ok {
		return p
	}
	return w
}

func autoDuration(d time.Duration) string {
	months := int(d.Hours()/24/30.44 + 0.5)
	if months < 1 {
		months = 1
	}
	y, m := months/12, months%12
	switch {
	case y == 0:
		return fmt.Sprintf("%d %s", m, plural(m, "mês", "meses"))
	case m == 0:
		return fmt.Sprintf("%d %s", y, plural(y, "ano", "anos"))
	}
	return fmt.Sprintf("%d %s e %d %s", y, plural(y, "ano", "anos"), m, plural(m, "mês", "meses"))
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// autoJobText descreve uma vaga para a IA: título, empresa, local, nível estimado,
// o cálculo da stack (o que cobre e o que falta) e a descrição.
func autoJobText(i int, j DiscoveredJob, desc string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[%d] %s", i, j.Title)
	if j.Company != "" {
		fmt.Fprintf(&b, " — %s", j.Company)
	}
	b.WriteString("\n")
	local := []string{}
	if j.International {
		local = append(local, "empresa no exterior")
	}
	if j.WorkplaceType != "" {
		local = append(local, autoPlace(j.WorkplaceType))
	}
	if j.Location != "" {
		local = append(local, j.Location)
	}
	if len(local) > 0 {
		fmt.Fprintf(&b, "Local: %s\n", strings.Join(local, ", "))
	}
	if len(j.Seniority) > 0 {
		lv := make([]string, 0, len(j.Seniority))
		for _, l := range j.Seniority {
			if n, ok := autoLevelNames[l]; ok {
				lv = append(lv, n)
			}
		}
		if len(lv) > 0 {
			fmt.Fprintf(&b, "Nível pelo título: %s\n", strings.Join(lv, "/"))
		}
	}
	if j.Language == "en" {
		b.WriteString("Vaga escrita em inglês\n")
	}
	if len(j.fit.Items) > 0 {
		var has, lacks []string
		for _, it := range j.fit.Items {
			switch it.Status {
			case jobfit.StatusStrong, jobfit.StatusPartial:
				has = append(has, it.Name)
			default:
				lacks = append(lacks, it.Name)
			}
		}
		fmt.Fprintf(&b, "Stack citada: o candidato tem %s", orNone(has))
		if len(lacks) > 0 {
			fmt.Fprintf(&b, "; não tem %s", strings.Join(lacks, ", "))
		}
		b.WriteString("\n")
	}
	desc = strings.TrimSpace(desc)
	if utf8.RuneCountInString(desc) > autoSelectDescMax {
		desc = string([]rune(desc)[:autoSelectDescMax]) + " […]"
	}
	if desc == "" {
		desc = "(sem descrição)"
	}
	fmt.Fprintf(&b, "Descrição:\n%s\n", desc)
	return b.String()
}

func orNone(s []string) string {
	if len(s) == 0 {
		return "nenhuma"
	}
	return strings.Join(s, ", ")
}

// autoQualityRank: alta 3, média 2, baixa 1.
func autoQualityRank(q db.JobQuality) int {
	switch q {
	case db.JobQualityHigh:
		return 3
	case db.JobQualityMid:
		return 2
	case db.JobQualityLow:
		return 1
	}
	return 0
}

// autoCandidate: uma vaga lida pela IA e o julgamento (nil = a IA não respondeu).
type autoCandidate struct {
	Job     DiscoveredJob
	Verdict *autoVerdict
}

// autoChoose escolhe até n: aprovadas pela IA com nota mínima, da maior nota para a
// menor (empate: a mais compatível pela stack).
func autoChoose(cands []autoCandidate, minScore, n int) []int {
	idx := []int{}
	for i, c := range cands {
		if c.Verdict != nil && c.Verdict.Apply && c.Verdict.Score >= minScore {
			idx = append(idx, i)
		}
	}
	sort.SliceStable(idx, func(a, b int) bool {
		va, vb := cands[idx[a]].Verdict, cands[idx[b]].Verdict
		if va.Score != vb.Score {
			return va.Score > vb.Score
		}
		return cands[idx[a]].Job.fit.CoveragePct > cands[idx[b]].Job.fit.CoveragePct
	})
	if len(idx) > n {
		idx = idx[:n]
	}
	return idx
}
