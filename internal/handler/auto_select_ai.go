package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"hackton-treino/internal/db"
	"hackton-treino/internal/jobfit"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
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
- Skill em nível "básico" não é domínio: não diga que o candidato "tem experiência" com ela e não conte como núcleo atendido.
- Plataforma ou tecnologia que dá nome à vaga ou é pedida como obrigatória (ex.: "Especialista VTEX", "Desenvolvedor PHP", Salesforce, SAP) e que o perfil não tem, ou tem só no nível básico, é requisito eliminatório.
- Vaga de área diferente da área principal do perfil (deduza pelos cargos e pelas skills avançadas/expert: ex. mobile, front-end, QA, dados, infra/SRE para um perfil backend): nota no máximo 55, a não ser que o perfil tenha nível avançado ou expert nas tecnologias centrais da vaga.
- Senioridade: compare os anos pedidos com o tempo total de experiência; pedir mais que 1,5× o tempo do perfil é eliminatório.
- "nota" de 0 a 100: 80+ forte; 60–79 vale tentar; 40–59 arriscada; abaixo de 40 não vale.
- Fatos da vaga (copie do texto, não julgue):
  - "anos_pedidos": o mínimo de anos de experiência que a vaga pede (número), ou null se não diz.
  - "modelo": "remoto", "híbrido" ou "presencial" conforme o texto da vaga; "" se não diz.
  - "faltando": lista do que a vaga marca como obrigatório (tecnologia, plataforma, certificação, idioma, formação, domínio de negócio) e o perfil não tem ou tem só no nível básico. Não inclua o que é "desejável", "diferencial" ou "nice to have". [] quando nada falta.
- "eliminatorio": outro requisito eliminatório que falha (cidadania, visto, estágio, função não técnica), em poucas palavras; "" quando nenhum falha.
- "candidatar": true só com nota 60 ou mais, "eliminatorio" vazio e no máximo 1 item em "faltando". A nota e o motivo precisam concordar com essa decisão; o que está em "faltando" não vira só um "Risco".
- "motivo": uma frase em português, até 160 caracteres, coerente com "candidatar":
  - candidatar true: comece pelo que faz valer a pena e, se houver, termine com o principal risco depois de "Risco:" (ex.: "Go e AWS são o núcleo da vaga e o forte do perfil. Risco: pede 3+ anos de Node.js.").
  - candidatar false: o fator que elimina ou enfraquece a candidatura (ex.: "Pede 5+ anos com Kotlin; o perfil tem 2 anos em Go.").
- Avalie todas as vagas da lista, sem pular nenhuma, usando o número entre colchetes.

Responda APENAS com JSON válido, sem texto antes ou depois:
{"avaliacoes":[{"vaga":1,"anos_pedidos":3,"modelo":"remoto","faltando":["Node.js"],"eliminatorio":"","nota":70,"candidatar":true,"motivo":"..."}]}`

// autoVerdict: o julgamento da IA sobre uma vaga.
type autoVerdict struct {
	Score  int
	Apply  bool
	Reason string
	// Blocker: requisito eliminatório que a IA apontou. Com ele, a vaga não entra,
	// mesmo que o modelo tenha marcado "candidatar" (já aconteceu: nota 75 com
	// "a vaga exige VTEX, que o candidato não possui").
	Blocker string
	// Fatos copiados da vaga pela IA; as regras de autoApplyRules decidem com eles.
	YearsAsked float64  // 0 = não diz
	Workplace  string   // remote, hybrid, onsite ou "" (não diz)
	Missing    []string // obrigatórios que o perfil não tem
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
			Vaga         flexInt         `json:"vaga"`
			Nota         flexInt         `json:"nota"`
			Candidatar   flexBool        `json:"candidatar"`
			Motivo       string          `json:"motivo"`
			Eliminatorio string          `json:"eliminatorio"`
			AnosPedidos  json.RawMessage `json:"anos_pedidos"`
			Modelo       string          `json:"modelo"`
			Faltando     json.RawMessage `json:"faltando"`
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
		blocker := strings.TrimSpace(a.Eliminatorio)
		switch strings.ToLower(strings.Trim(blocker, ".")) {
		case "nenhum", "nenhuma", "não", "nao", "-", "n/a", "none", "null":
			blocker = ""
		}
		if utf8.RuneCountInString(blocker) > 120 {
			blocker = string([]rune(blocker)[:120]) + "…"
		}
		if reason == "" && blocker != "" {
			reason = "Eliminatório: " + blocker
		}
		res[i] = autoVerdict{
			Score: score, Apply: bool(a.Candidatar) && blocker == "", Reason: reason, Blocker: blocker,
			YearsAsked: rawYears(a.AnosPedidos), Workplace: rawWorkplace(a.Modelo), Missing: rawList(a.Faltando),
		}
	}
	if len(res) == 0 {
		return nil, errors.New("nenhuma avaliação na resposta")
	}
	return res, nil
}

// rawYears aceita 3, 3.5, "3", "3+", "3 anos"; o resto (null, "") vira 0.
func rawYears(b json.RawMessage) float64 {
	s := strings.Trim(strings.TrimSpace(string(b)), `"`)
	end := 0
	for end < len(s) && (s[end] >= '0' && s[end] <= '9' || s[end] == '.' || s[end] == ',') {
		end++
	}
	v, err := strconv.ParseFloat(strings.ReplaceAll(s[:end], ",", "."), 64)
	if err != nil || v < 0 || v > 40 {
		return 0
	}
	return v
}

func rawWorkplace(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	switch {
	case strings.HasPrefix(s, "remot"), strings.HasPrefix(s, "home"):
		return "remote"
	case strings.HasPrefix(s, "híbrid"), strings.HasPrefix(s, "hibrid"), strings.HasPrefix(s, "hybrid"):
		return "hybrid"
	case strings.HasPrefix(s, "presen"), s == "onsite", s == "on-site":
		return "onsite"
	}
	return ""
}

// rawList aceita ["a","b"] ou "a, b".
func rawList(b json.RawMessage) []string {
	var list []string
	if json.Unmarshal(b, &list) != nil {
		var s string
		if json.Unmarshal(b, &s) != nil {
			return nil
		}
		list = strings.Split(s, ",")
	}
	out := make([]string, 0, len(list))
	for _, it := range list {
		it = strings.TrimSpace(it)
		switch strings.ToLower(it) {
		case "", "nenhum", "nenhuma", "-", "n/a", "none":
			continue
		}
		out = append(out, it)
	}
	return out
}

// autoApplyRules: as regras de corte ficam no código, não no julgamento do modelo. Na
// 2ª rodada o modelo aprovou vagas apontando ele mesmo "Risco: presencial/híbrida" e
// "pede Ruby on Rails e Vue.js, que o candidato não possui". Aqui:
//   - anos pedidos acima de 1,5× a carreira do perfil → fora;
//   - modelo de trabalho fora das preferências (só quando o site não informou; se
//     informou, o Descobrir já filtrou) → fora;
//   - 2+ obrigatórios faltando → fora; 1 faltando → nota no máximo 65.
//
// years < 0: tempo de carreira desconhecido.
func autoApplyRules(v *autoVerdict, j DiscoveredJob, years float64, places []string) {
	drop := func(why string) {
		v.Apply = false
		v.Reason = "Fora: " + why + "."
	}
	switch {
	case v.Blocker != "":
		v.Apply = false
	case years > 0 && v.YearsAsked > years*1.5:
		drop(fmt.Sprintf("pede %s+ anos; o perfil tem %s", trimYears(v.YearsAsked), autoDuration(time.Duration(years*365.25*24)*time.Hour)))
	case len(places) > 0 && j.WorkplaceType == "" && v.Workplace != "" && !slices.Contains(places, v.Workplace):
		drop("vaga " + autoPlace(v.Workplace) + ", fora do local que você escolheu")
	case len(v.Missing) >= 2:
		drop("obrigatórios que o perfil não tem: " + strings.Join(v.Missing[:min(len(v.Missing), 4)], ", "))
	case len(v.Missing) == 1:
		v.Score = min(v.Score, 65)
	}
}

func trimYears(y float64) string {
	return strings.TrimSuffix(strconv.FormatFloat(y, 'f', 1, 64), ".0")
}

// autoNonTech: título claramente fora de tecnologia (vendas, RH...): nem vai para a IA.
var autoNonTech = []string{"account executive", "account manager", "sales", "vendas", "vendedor", "comercial",
	"customer success", "recruiter", "recrutador", "talent acquisition", "business development", "marketing manager"}

func autoIsNonTech(title string) bool {
	t := strings.ToLower(title)
	for _, w := range autoNonTech {
		if strings.Contains(t, w) {
			return true
		}
	}
	return false
}

// autoExperience: tempo total de carreira, sem contar duas vezes empregos simultâneos.
// ok=false quando nenhuma experiência tem data de início.
func autoExperience(exps []db.UserExperience, now time.Time) (time.Duration, bool) {
	type span struct{ from, to time.Time }
	spans := make([]span, 0, len(exps))
	for _, e := range exps {
		if !e.StartDate.Valid {
			continue
		}
		to := now
		if !e.IsCurrentJob && e.EndDate.Valid {
			to = e.EndDate.Time
		}
		if to.After(e.StartDate.Time) {
			spans = append(spans, span{e.StartDate.Time, to})
		}
	}
	if len(spans) == 0 {
		return 0, false
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].from.Before(spans[j].from) })
	var total time.Duration
	cur := spans[0]
	for _, s := range spans[1:] {
		switch {
		case s.from.After(cur.to):
			total += cur.to.Sub(cur.from)
			cur = s
		case s.to.After(cur.to):
			cur.to = s.to
		}
	}
	return total + cur.to.Sub(cur.from), true
}

// autoLevelFit: quando a pessoa não escolheu nível no Descobrir, o nível vem do tempo de
// carreira. "Alvo" vai primeiro para a IA; "esticar" completa a lista; o resto nem
// é lido (na 1ª rodada, 14 de 20 vagas lidas eram sênior 5+/8+ anos, principal ou
// estágio para um perfil de 3 anos e meio: todas recusadas, gastando a IA à toa).
// Vaga sem nível informado conta como alvo.
func autoLevelFit(years float64) (target, stretch []string) {
	switch {
	case years < 1:
		return []string{"intern", "junior"}, []string{"mid"}
	case years < 3:
		return []string{"junior", "mid"}, []string{"senior"}
	case years < 6:
		return []string{"mid"}, []string{"junior", "senior"}
	case years < 10:
		return []string{"senior"}, []string{"mid", "expert"}
	default:
		return []string{"senior", "expert"}, []string{"mid"}
	}
}

// autoLevelRank: 0 = alvo, 1 = esticar, -1 = fora. Vaga com vários níveis ("All
// levels") vale pelo melhor deles.
func autoLevelRank(levels, target, stretch []string) int {
	if len(levels) == 0 {
		return 0
	}
	// Estágio é outra trilha (pede matrícula em curso): fora, a menos que seja o alvo,
	// mesmo quando a vaga também cita júnior.
	if slices.Contains(levels, "intern") && !slices.Contains(target, "intern") {
		return -1
	}
	best := -1
	for _, l := range levels {
		switch {
		case slices.Contains(target, l):
			return 0
		case slices.Contains(stretch, l):
			best = 1
		}
	}
	return best
}

// autoDedup: a mesma vaga publicada mais de uma vez com a empresa escrita diferente
// ("MEP | Wellhub (Gympass)" e "MEP | Wellhub"; "Méliuz" e "Carreiras Méliuz") só é
// lida uma vez: mesmo título e alguma palavra da empresa em comum.
type autoDedup map[string][]map[string]bool

var autoCompanyNoise = map[string]bool{"carreiras": true, "careers": true, "grupo": true, "group": true,
	"inc": true, "ltda": true, "sa": true, "brasil": true, "brazil": true, "the": true, "de": true, "do": true, "da": true}

func autoWords(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

// seen registra a vaga e diz se ela já tinha aparecido.
func (d autoDedup) seen(title, company string) bool {
	key := strings.Join(autoWords(title), " ")
	words := map[string]bool{}
	for _, w := range autoWords(company) {
		if !autoCompanyNoise[w] && len(w) > 1 {
			words[w] = true
		}
	}
	for _, prev := range d[key] {
		if len(words) == 0 && len(prev) == 0 {
			return true
		}
		for w := range words {
			if prev[w] {
				return true
			}
		}
	}
	d[key] = append(d[key], words)
	return false
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
		}
		fmt.Fprintf(&b, "- %s em %s (%s)", e.JobRole, e.CompanyName, period)
		if len(e.TechStack) > 0 {
			fmt.Fprintf(&b, "; stack: %s", strings.Join(e.TechStack, ", "))
		}
		b.WriteString("\n")
	}
	if total, ok := autoExperience(exps, now); ok {
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
		b.WriteString("- nível: não escolhido (julgue pelo tempo de experiência)\n")
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
