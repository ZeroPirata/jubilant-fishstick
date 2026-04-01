package worker

import (
	"encoding/json"
	"fmt"
	"hackton-treino/internal/db"
	"hackton-treino/internal/services"
)

func (w *Worker) buildUserPrompt(vaga *db.Vaga, match *services.MatchResult) (string, error) {
	vagaP := vagaPrompt{
		Empresa:    vaga.Empresa.String,
		Titulo:     vaga.Titulo.String,
		Stack:      vaga.Stack,
		Requisitos: vaga.Requisitos,
	}
	if len(vaga.Requisitos) == 0 {
		vagaP.Descricao = vaga.Descricao.String
	}

	excelentes := make([]string, 0, len(match.Excelentes))
	for _, e := range match.Excelentes {
		excelentes = append(excelentes, string(e))
	}

	comentarios := make([]string, 0, len(match.Feedbacks))
	for _, f := range match.Feedbacks {
		comentarios = append(comentarios, f.String)
	}

	prompt := userPrompt{
		Vaga:         vagaP,
		Experiencias: make([]experienciaPrompt, 0, len(match.Experiencias)),
		Habilidades:  make([]habilidadePrompt, 0, len(match.Habilidades)),
		Projetos:     make([]projetoPrompt, 0, len(match.Projetos)),
		Formacoes:    make([]formacaoPrompt, 0, len(match.Formacoes)),
		Feedback: feedbackPrompt{
			ExemplosExcelentes:    excelentes,
			ComentariosAnteriores: comentarios,
		},
		// Certificacoes: make([]certificacaoPrompt, 0, len(match.Certificacoes)),
	}

	for _, e := range match.Experiencias {
		conquistas := e.Conquistas
		if len(conquistas) > 4 {
			conquistas = conquistas[:4]
		}
		ep := experienciaPrompt{
			Empresa:    e.Empresa,
			Cargo:      e.Cargo,
			Descricao:  e.Descricao.String,
			Atual:      e.Atual,
			Stack:      e.Stack,
			Conquistas: conquistas,
		}
		if e.DataInicio.Valid {
			ep.DataInicio = fmt.Sprintf("%04d-%02d", e.DataInicio.Time.Year(), e.DataInicio.Time.Month())
		}
		if e.DataFim.Valid {
			ep.DataFim = fmt.Sprintf("%04d-%02d", e.DataFim.Time.Year(), e.DataFim.Time.Month())
		}
		prompt.Experiencias = append(prompt.Experiencias, ep)
	}

	for _, h := range match.Habilidades {
		prompt.Habilidades = append(prompt.Habilidades, habilidadePrompt{
			Nome:  h.Nome,
			Nivel: string(h.Nivel),
		})
	}

	for _, p := range match.Projetos {
		prompt.Projetos = append(prompt.Projetos, projetoPrompt{
			Nome:      p.Nome,
			Descricao: p.Descricao.String,
			Link:      p.Link.String,
		})
	}

	for _, f := range match.Formacoes {
		fp := formacaoPrompt{
			Instituicao: f.Instituicao,
			Curso:       f.Curso,
		}
		if f.DataInicio.Valid {
			fp.DataInicio = fmt.Sprintf("%04d-%02d", f.DataInicio.Time.Year(), f.DataInicio.Time.Month())
		}
		if f.DataFim.Valid {
			fp.DataFim = fmt.Sprintf("%04d-%02d", f.DataFim.Time.Year(), f.DataFim.Time.Month())
		}
		prompt.Formacoes = append(prompt.Formacoes, fp)
	}

	// for _, c := range match.Certificacoes {
	// 	cp := certificacaoPrompt{
	// 		Nome:    c.Nome,
	// 		Emissor: c.Emissor,
	// 	}
	// 	if c.EmitidoEm.Valid {
	// 		cp.EmitidoEm = fmt.Sprintf("%04d-%02d", c.EmitidoEm.Time.Year(), c.EmitidoEm.Time.Month())
	// 	}
	// 	if c.Link.Valid {
	// 		cp.Link = c.Link.String
	// 	}
	// 	prompt.Certificacoes = append(prompt.Certificacoes, cp)
	// }

	raw, err := json.Marshal(prompt)
	if err != nil {
		return "", fmt.Errorf("build prompt: %w", err)
	}

	return string(raw), nil
}
