package agent

import "strings"

// SkillSelector scores skills against user input via lexical trigger matching
// and returns the top-K highest-scoring skills.
type SkillSelector struct{}

// SkillScore holds a skill and its match score.
type SkillScore struct {
	Skill *Skill
	Score int
}

// SelectSkills scores all skills against the input and returns up to maxSkills
// top scorers. When maxSkills <= 0, defaults to 3. Returns nil when no skills
// score above zero.
func (s *SkillSelector) SelectSkills(input string, skills []Skill, maxSkills int) []SkillScore {
	if maxSkills <= 0 {
		maxSkills = 3
	}
	if len(skills) == 0 {
		return nil
	}
	lower := strings.ToLower(input)
	var scored []SkillScore
	for i := range skills {
		score := scoreSkill(&skills[i], lower)
		if score > 0 {
			scored = append(scored, SkillScore{Skill: &skills[i], Score: score})
		}
	}
	if len(scored) == 0 {
		return nil
	}
	// Sort descending by score (bubble sort for simplicity, skills count is small)
	for i := 0; i < len(scored)-1; i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].Score > scored[i].Score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}
	if len(scored) > maxSkills {
		scored = scored[:maxSkills]
	}
	return scored
}

func scoreSkill(skill *Skill, lowerInput string) int {
	score := 0
	for _, trigger := range skill.Triggers {
		if strings.Contains(lowerInput, strings.ToLower(trigger)) {
			score++
		}
	}
	return score
}
