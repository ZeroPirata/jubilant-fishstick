package handler

import (
	"context"
	"hackton-treino/internal/repository/users"
)

func checkProfileWeakness(ctx context.Context, repo users.Repository, userID string) []string {
	var warnings []string

	exps, _ := repo.QuerySelectAllExperiences(ctx, userID)
	skills, _ := repo.QuerySelectAllSkills(ctx, userID)
	projs, _ := repo.QuerySelectAllProjects(ctx, userID)

	if len(skills) == 0 {
		warnings = append(warnings, "Nenhuma habilidade técnica cadastrada — adicione suas skills para melhorar o match com a vaga.")
	}

	if len(exps) == 0 && len(projs) == 0 {
		warnings = append(warnings, "Perfil sem experiências profissionais e sem projetos — o currículo será baseado apenas em habilidades e formação.")
		return warnings
	}

	thinExps := 0
	for _, e := range exps {
		if e.Description.String == "" && len(e.Achievements) == 0 {
			thinExps++
		}
	}
	if len(exps) > 0 && thinExps == len(exps) {
		warnings = append(warnings, "Suas experiências não têm conquistas nem descrição — adicione detalhes para um currículo mais forte.")
	}

	thinProjs := 0
	for _, p := range projs {
		if p.Description == "" {
			thinProjs++
		}
	}
	if len(projs) > 0 && thinProjs == len(projs) {
		warnings = append(warnings, "Seus projetos não têm descrição técnica — adicione detalhes para destacar suas contribuições.")
	}

	return warnings
}
