package worker

import (
	"hackton-treino/internal/db"
	"hackton-treino/internal/sse"
	"hackton-treino/internal/util"
	"strings"
)

func analisarGap(stack []string, skills []db.UserSkill, aliases map[string]string) sse.GapAnalysis {
	if len(stack) == 0 {
		return sse.GapAnalysis{CoveragePct: 100}
	}

	// Normaliza skill_name + tags para matching mais preciso
	var normalized []string
	for _, s := range skills {
		f := strings.TrimSpace(strings.ToLower(s.SkillName))
		if canonical, ok := aliases[f]; ok {
			f = canonical
		}
		if f != "" {
			normalized = append(normalized, f)
		}
		// Inclui tags na normalização
		for _, tag := range s.Tags {
			t := strings.TrimSpace(strings.ToLower(tag))
			if t == "" {
				continue
			}
			if canonical, ok := aliases[t]; ok {
				t = canonical
			}
			normalized = append(normalized, t)
		}
	}

	var missing []string
	matched := 0
	for _, item := range stack {
		itemNorm := util.Normalize(item)
		found := false
		for _, filtro := range normalized {
			if techMatch(itemNorm, filtro) {
				found = true
				break
			}
		}
		if found {
			matched++
		} else {
			missing = append(missing, item)
		}
	}

	coverage := 0
	if len(stack) > 0 {
		coverage = (matched * 100) / len(stack)
	}

	return sse.GapAnalysis{MissingSkills: missing, CoveragePct: coverage}
}
