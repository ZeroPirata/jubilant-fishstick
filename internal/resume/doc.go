// Package resume separa o CONTEÚDO do currículo (escrito pela IA) da APRESENTAÇÃO
// (montada pelo código). A IA devolve só texto livre — resumo, bullets, descrições,
// agrupamento de habilidades e a carta —, referenciando experiências e projetos por id.
// Empresa, cargo, datas, contato e formação vêm direto do banco. O resultado é um Doc,
// que qualquer template renderiza sem nova chamada à IA.
package resume

import "time"

// DocVersion identifica o formato estruturado em generated_resumes.content_json.
// Conteúdos sem "version" são do formato antigo (texto puro).
const DocVersion = 2

type Doc struct {
	Version        int             `json:"version"`
	Lang           string          `json:"lang"` // "pt-br" | "en"
	Header         Header          `json:"header"`
	Summary        string          `json:"summary"`
	Experiences    []Experience    `json:"experiences"`
	Skills         []SkillGroup    `json:"skills"`
	Projects       []Project       `json:"projects"`
	Education      []Education     `json:"education"`
	Certifications []Certification `json:"certifications"`
	Languages      []Language      `json:"languages,omitempty"`
}

type Header struct {
	Name      string `json:"name"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	Linkedin  string `json:"linkedin"`
	Github    string `json:"github"`
	Portfolio string `json:"portfolio"`
}

// Datas em "AAAA-MM"; o template decide como exibir.
type Experience struct {
	Company string   `json:"company"`
	Role    string   `json:"role"`
	Start   string   `json:"start"`
	End     string   `json:"end,omitempty"`
	Current bool     `json:"current"`
	Bullets []string `json:"bullets"`
}

type SkillGroup struct {
	Category string   `json:"category"`
	Items    []string `json:"items"`
}

type Project struct {
	Name        string `json:"name"`
	URL         string `json:"url,omitempty"`
	Description string `json:"description"`
}

type Education struct {
	Institution string `json:"institution"`
	Course      string `json:"course"`
	Start       string `json:"start,omitempty"`
	End         string `json:"end,omitempty"`
}

type Certification struct {
	Name   string `json:"name"`
	Issuer string `json:"issuer"`
	Date   string `json:"date,omitempty"`
	URL    string `json:"url,omitempty"`
}

// Language já vem com nome e nível no idioma do documento ("Inglês", "Avançado").
type Language struct {
	Name  string `json:"name"`
	Level string `json:"level"`
}

// LLMOutput é o JSON que a IA devolve (schema descrito no system prompt).
type LLMOutput struct {
	Resumo       string `json:"resumo"`
	Experiencias []struct {
		ID      int      `json:"id"`
		Bullets []string `json:"bullets"`
	} `json:"experiencias"`
	Habilidades []struct {
		Categoria string   `json:"categoria"`
		Itens     []string `json:"itens"`
	} `json:"habilidades"`
	Projetos []struct {
		ID        int    `json:"id"`
		Descricao string `json:"descricao"`
	} `json:"projetos"`
	CoverLetter string `json:"cover_letter"`
	Error       string `json:"error,omitempty"`
}

// Source é o que o banco sabe do candidato, na mesma ordem (ids) enviada à IA.
type Source struct {
	Lang           string
	Header         Header
	Experiences    []SourceExperience
	Projects       []SourceProject
	Education      []Education
	Certifications []Certification
	Languages      []Language // já traduzidos para Lang
	Skills         []string
	JobStack       []string          // stack da vaga: usada para detectar tecnologia que o candidato não tem
	Aliases        map[string]string // stack_aliases: variação → nome canônico
}

type SourceExperience struct {
	Company      string
	Role         string
	Description  string
	Start        time.Time
	End          time.Time // zero se atual
	Current      bool
	Stack        []string
	Achievements []string
}

type SourceProject struct {
	Name        string
	Description string
	URL         string
}

// MonthYear formata time.Time como "AAAA-MM" (vazio para zero).
func MonthYear(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01")
}
