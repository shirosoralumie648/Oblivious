package agent

import "testing"

func TestSkillSelector_SelectSkills_NoSkills(t *testing.T) {
	selector := &SkillSelector{}
	result := selector.SelectSkills("input", nil, 3)
	if result != nil {
		t.Fatalf("expected nil for empty skills, got %v", result)
	}
}

func TestSkillSelector_SelectSkills_NoMatch(t *testing.T) {
	selector := &SkillSelector{}
	skills := []Skill{
		{Name: "coding", Triggers: []string{"code", "debug"}},
		{Name: "math", Triggers: []string{"calculate", "sum"}},
	}
	result := selector.SelectSkills("general query", skills, 3)
	if result != nil {
		t.Fatalf("expected nil when no triggers match, got %v", result)
	}
}

func TestSkillSelector_SelectSkills_ScoresAndSorts(t *testing.T) {
	selector := &SkillSelector{}
	skills := []Skill{
		{Name: "coding", Triggers: []string{"code", "debug"}},
		{Name: "math", Triggers: []string{"calculate", "sum"}},
		{Name: "writing", Triggers: []string{"write", "edit"}},
	}
	input := "help me code and debug and calculate this sum"
	result := selector.SelectSkills(input, skills, 3)
	if len(result) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(result))
	}
	if result[0].Skill.Name != "coding" && result[0].Skill.Name != "math" {
		t.Fatalf("expected coding or math first, got %s", result[0].Skill.Name)
	}
	if result[0].Score != 2 {
		t.Fatalf("expected top score 2, got %d", result[0].Score)
	}
	if result[1].Score != 2 {
		t.Fatalf("expected second score 2, got %d", result[1].Score)
	}
}

func TestSkillSelector_SelectSkills_LimitsTopK(t *testing.T) {
	selector := &SkillSelector{}
	skills := []Skill{
		{Name: "a", Triggers: []string{"alpha"}},
		{Name: "b", Triggers: []string{"beta"}},
		{Name: "c", Triggers: []string{"gamma"}},
	}
	input := "alpha beta gamma"
	result := selector.SelectSkills(input, skills, 2)
	if len(result) != 2 {
		t.Fatalf("expected maxSkills=2 to limit result, got %d", len(result))
	}
}

func TestSkillSelector_SelectSkills_DefaultsMaxSkills(t *testing.T) {
	selector := &SkillSelector{}
	skills := []Skill{
		{Name: "a", Triggers: []string{"one"}},
		{Name: "b", Triggers: []string{"two"}},
		{Name: "c", Triggers: []string{"three"}},
		{Name: "d", Triggers: []string{"four"}},
	}
	input := "one two three four"
	result := selector.SelectSkills(input, skills, 0)
	if len(result) != 3 {
		t.Fatalf("expected default maxSkills=3, got %d", len(result))
	}
}

func TestSkillSelector_SelectSkills_CaseInsensitive(t *testing.T) {
	selector := &SkillSelector{}
	skills := []Skill{
		{Name: "coding", Triggers: []string{"Code", "DEBUG"}},
	}
	input := "help me code and debug"
	result := selector.SelectSkills(input, skills, 3)
	if len(result) != 1 || result[0].Score != 2 {
		t.Fatalf("expected case-insensitive match score 2, got %v", result)
	}
}

func TestSkillSelector_SelectSkills_TieBreaker(t *testing.T) {
	selector := &SkillSelector{}
	skills := []Skill{
		{Name: "x", Triggers: []string{"key"}},
		{Name: "y", Triggers: []string{"key"}},
	}
	input := "key"
	result := selector.SelectSkills(input, skills, 1)
	if len(result) != 1 || result[0].Score != 1 {
		t.Fatalf("expected 1 result with score 1, got %v", result)
	}
}
