package workflows

import "strings"

// TranscriptEntry is one optional workflow-eval trace event.
type TranscriptEntry struct {
	Step    string `json:"step"`
	Details string `json:"details,omitempty"`
}

// EvalResult describes one deterministic workflow fixture result.
type EvalResult struct {
	Name       string            `json:"name"`
	Passed     bool              `json:"passed"`
	Plan       PlanResult        `json:"plan"`
	Checks     []EvalCheck       `json:"checks,omitempty"`
	Failures   []string          `json:"failures,omitempty"`
	Transcript []TranscriptEntry `json:"transcript,omitempty"`
}

// EvalCheck describes one scored expectation for a workflow fixture.
type EvalCheck struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Details string `json:"details,omitempty"`
}

// Scorecard summarizes workflow fixture quality.
type Scorecard struct {
	Total   int          `json:"total"`
	Passed  int          `json:"passed"`
	Failed  int          `json:"failed"`
	Results []EvalResult `json:"results"`
}

// RunEvalFixtures evaluates workflow planning against deterministic fixtures.
func RunEvalFixtures(ctx Context, captureTranscript bool) Scorecard {
	fixtures := EvalFixtures()
	scorecard := Scorecard{Total: len(fixtures)}
	for _, fixture := range fixtures {
		result := EvalResult{Name: fixture.Name}
		if captureTranscript {
			result.Transcript = append(result.Transcript, TranscriptEntry{Step: "fixture", Details: fixture.Task})
		}
		plan, ok := Plan(ctx, PlanRequest{Task: fixture.Task, App: fixture.App})
		result.Plan = plan
		if captureTranscript {
			result.Transcript = append(result.Transcript, TranscriptEntry{Step: "workflow", Details: plan.WorkflowID})
		}
		result.addCheck("plan returned", ok, "Atlas should return a workflow plan for the selected app")
		result.addCheck("workflow id", plan.WorkflowID == fixture.WantWorkflowID, "got "+plan.WorkflowID+", want "+fixture.WantWorkflowID)
		result.addCheck("app selection", plan.App == fixture.expectedApp(), "got "+plan.App+", want "+fixture.expectedApp())
		for _, part := range fixture.commandParts() {
			result.addCheck("command contains "+part, stringSliceContainsPart(plan.Commands, part), "commands should include "+part)
		}
		for _, part := range fixture.fileParts() {
			result.addCheck("file contains "+part, stringSliceContainsPart(plan.Files, part), "files should include "+part)
		}
		for _, part := range fixture.AvoidFileParts {
			result.addCheck("file avoids "+part, !stringSliceContainsPart(plan.Files, part), "files must not include "+part)
		}
		for _, path := range fixture.docsPaths() {
			result.addCheck("docs include "+path, docRefsContain(plan.Docs, path), "docs should include "+path)
		}
		for _, tool := range fixture.WantTools {
			result.addCheck("tool includes "+tool, stringSliceContains(plan.Tools, tool), "tools should include "+tool)
		}
		for _, part := range fixture.WantValidationParts {
			result.addCheck("validation contains "+part, validationContainsPart(plan.Verification, part), "validation should include "+part)
		}
		for _, part := range fixture.WantWarningParts {
			result.addCheck("warning contains "+part, stringSliceContainsPart(plan.Warnings, part), "warnings should include "+part)
		}
		for _, file := range plan.Files {
			policy := FilePolicy(FilePolicyRequest{Path: file})
			result.addCheck("ownership allows "+file, policy.Editable, file+" classified as "+policy.Classification+" through "+policy.ChangeThrough)
		}
		if captureTranscript {
			for _, check := range result.Checks {
				status := "pass"
				if !check.Passed {
					status = "fail"
				}
				result.Transcript = append(result.Transcript, TranscriptEntry{Step: "check:" + status, Details: check.Name})
			}
		}
		result.Passed = len(result.Failures) == 0
		if result.Passed {
			scorecard.Passed++
		} else {
			scorecard.Failed++
		}
		scorecard.Results = append(scorecard.Results, result)
	}
	return scorecard
}

func (r *EvalResult) addCheck(name string, passed bool, details string) {
	check := EvalCheck{Name: name, Passed: passed, Details: details}
	r.Checks = append(r.Checks, check)
	if !passed {
		r.Failures = append(r.Failures, name+": "+details)
	}
}

func (f EvalFixture) expectedApp() string {
	if strings.TrimSpace(f.App) != "" {
		return f.App
	}
	return "app"
}

func (f EvalFixture) commandParts() []string {
	return combined(f.WantCommandPart, f.WantCommandParts)
}

func (f EvalFixture) fileParts() []string {
	return combined(f.WantFilePart, f.WantFileParts)
}

func (f EvalFixture) docsPaths() []string {
	return combined(f.WantDocsPath, f.WantDocsPaths)
}

func combined(first string, rest []string) []string {
	out := []string{}
	if strings.TrimSpace(first) != "" {
		out = append(out, first)
	}
	return append(out, rest...)
}

func stringSliceContainsPart(values []string, want string) bool {
	for _, value := range values {
		if strings.Contains(value, want) {
			return true
		}
	}
	return false
}

func stringSliceContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func docRefsContain(values []DocReference, path string) bool {
	for _, value := range values {
		if value.Path == path {
			return true
		}
	}
	return false
}

func validationContainsPart(values []ValidationStep, want string) bool {
	for _, value := range values {
		if strings.Contains(value.Command, want) || strings.Contains(value.Purpose, want) {
			return true
		}
	}
	return false
}
