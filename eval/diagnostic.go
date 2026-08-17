package eval

import (
	"context"
	"fmt"
	"strings"
)

// GuidanceDiagnosticRequest identifies one paired control-versus-AGENTS diagnostic trial.
type GuidanceDiagnosticRequest struct {
	LogicalTrialID  string
	Definition      EvaluationDefinition
	DestinationRoot string
	ForjExecutable  string
	Environments    map[string][]string
	Runtime         RuntimeIdentity
}

// GuidanceDiagnosticAttempt retains one treatment's result and any operational error independently.
type GuidanceDiagnosticAttempt struct {
	Profile string        `json:"profile"`
	Result  AttemptResult `json:"result"`
	Error   string        `json:"error,omitempty"`
}

// GuidanceDiagnosticResult contains both treatments even when one attempt fails operationally.
type GuidanceDiagnosticResult struct {
	LogicalTrialID string                      `json:"logical_trial_id"`
	Attempts       []GuidanceDiagnosticAttempt `json:"attempts"`
}

// RunGuidanceDiagnostic runs isolated control and AGENTS treatments against the same promoted definition.
func (runner Runner) RunGuidanceDiagnostic(ctx context.Context, request GuidanceDiagnosticRequest) (GuidanceDiagnosticResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if !artifactAttemptIDPattern.MatchString(request.LogicalTrialID) {
		return GuidanceDiagnosticResult{}, fmt.Errorf("logical trial ID %q must be a safe slug", request.LogicalTrialID)
	}
	if strings.TrimSpace(request.DestinationRoot) == "" || strings.TrimSpace(request.ForjExecutable) == "" {
		return GuidanceDiagnosticResult{}, fmt.Errorf("diagnostic preparation inputs are incomplete")
	}
	profiles := []string{GuidanceProfileNone, GuidanceProfileAgents}
	for _, profile := range profiles {
		environment, ok := request.Environments[profile]
		if !ok || environment == nil {
			return GuidanceDiagnosticResult{}, fmt.Errorf("diagnostic environment for profile %q is required", profile)
		}
	}
	result := GuidanceDiagnosticResult{LogicalTrialID: request.LogicalTrialID}
	for _, profile := range profiles {
		environment := request.Environments[profile]
		attemptID := request.LogicalTrialID + "-" + profile
		attempt, err := runner.Run(ctx, AttemptRequest{
			AttemptID:      attemptID,
			LogicalTrialID: request.LogicalTrialID,
			Intent:         IntentDiagnostic,
			Definition:     request.Definition,
			Preparation: PreparationRequest{
				ScenarioID:      request.Definition.ProjectScenario,
				DestinationRoot: request.DestinationRoot,
				ForjExecutable:  request.ForjExecutable,
				OrchestrationID: attemptID,
				Environment:     append([]string(nil), environment...),
			},
			GuidanceProfile: profile,
			Runtime:         request.Runtime,
		})
		record := GuidanceDiagnosticAttempt{Profile: profile, Result: attempt}
		if err != nil {
			record.Error = err.Error()
		}
		result.Attempts = append(result.Attempts, record)
		if err := ctx.Err(); err != nil {
			return result, err
		}
	}
	if len(result.Attempts) == 2 {
		left := result.Attempts[0].Result
		right := result.Attempts[1].Result
		if left.PlanDigest != "" && right.PlanDigest != "" && left.PlanDigest != right.PlanDigest {
			return result, fmt.Errorf("paired treatments received different preparation plans")
		}
		if left.PreparedTree != "" && right.PreparedTree != "" && left.PreparedTree != right.PreparedTree {
			return result, fmt.Errorf("paired treatments received different prepared trees")
		}
	}
	return result, nil
}
