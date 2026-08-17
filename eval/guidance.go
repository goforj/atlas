package eval

import (
	"context"
	"fmt"
	"strings"

	"github.com/goforj/atlas/guidelines"
	"github.com/goforj/atlas/project"
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
	return ResolveProjectGuidance(profileName, facts)
}

// Resolve creates one immutable treatment from the canonical Atlas guidance composer.
func ResolveProjectGuidance(profile string, facts project.Project) (Guidance, error) {
	switch strings.TrimSpace(profile) {
	case GuidanceProfileNone:
		return Guidance{Profile: GuidanceProfileNone, Files: map[string][]byte{}}, nil
	case GuidanceProfileAgents:
		body := strings.TrimSpace(guidelines.Compose(facts)) + "\n"
		return Guidance{Profile: GuidanceProfileAgents, Files: map[string][]byte{"AGENTS.md": []byte(body)}}, nil
	default:
		return Guidance{}, fmt.Errorf("unknown evaluation guidance profile %q", profile)
	}
}
