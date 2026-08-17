package eval

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/goforj/atlas/guidelines"
	"github.com/goforj/atlas/project"
	"github.com/goforj/atlas/skills"
)

// ProjectGuidanceResolver composes treatments from the exact prepared Project rather than caller-supplied facts.
type ProjectGuidanceResolver struct{}

// Resolve discovers the prepared Project and delegates to the canonical profile composer.
func (ProjectGuidanceResolver) Resolve(ctx context.Context, profileName string, preparation PreparationResult) (Guidance, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return Guidance{}, err
	}
	if strings.TrimSpace(preparation.ProjectRoot) == "" {
		return Guidance{}, fmt.Errorf("prepared Project root is required")
	}
	facts, err := project.Discover(preparation.ProjectRoot)
	if err != nil {
		return Guidance{}, fmt.Errorf("discover prepared Project: %w", err)
	}
	guidance, err := ResolveProjectGuidance(profileName, facts)
	if err != nil || len(guidance.Skills) == 0 {
		return guidance, err
	}
	projectSkills, err := skills.ProjectSkills(preparation.ProjectRoot)
	if err != nil {
		return Guidance{}, fmt.Errorf("discover prepared Project skills: %w", err)
	}
	for _, skill := range projectSkills {
		guidance.Skills = append(guidance.Skills, skill.Name)
	}
	sort.Strings(guidance.Skills)
	return guidance, nil
}

// Resolve creates one immutable treatment from the canonical Atlas guidance composer.
func ResolveProjectGuidance(profile string, facts project.Project) (Guidance, error) {
	switch strings.TrimSpace(profile) {
	case GuidanceProfileNone:
		return Guidance{Profile: GuidanceProfileNone, Files: map[string][]byte{}}, nil
	case GuidanceProfileAgents:
		body := strings.TrimSpace(guidelines.Compose(facts)) + "\n"
		return Guidance{Profile: GuidanceProfileAgents, Files: map[string][]byte{"AGENTS.md": []byte(body)}}, nil
	case GuidanceProfileAgentsSkills:
		body := strings.TrimSpace(guidelines.Compose(facts)) + "\n"
		return Guidance{Profile: GuidanceProfileAgentsSkills, Files: map[string][]byte{"AGENTS.md": []byte(body)}, Skills: skills.RecommendedNames(facts)}, nil
	case GuidanceProfileAtlas:
		body := strings.TrimSpace(guidelines.Compose(facts)) + "\n"
		return Guidance{Profile: GuidanceProfileAtlas, Files: map[string][]byte{"AGENTS.md": []byte(body)}, Skills: skills.RecommendedNames(facts), MCP: []string{"goforj-atlas"}}, nil
	default:
		return Guidance{}, fmt.Errorf("unknown evaluation guidance profile %q", profile)
	}
}
