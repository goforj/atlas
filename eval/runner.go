package eval

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const evaluationCleanupTimeout = 10 * time.Second

var authoritativeCapabilities = []Capability{
	CapabilityProcessCleanup,
	CapabilityCredentialIsolation,
	CapabilityHostFilesystemIsolation,
	CapabilityNetworkEnforcement,
	CapabilityVerifierIsolation,
	CapabilityArtifactIsolation,
}

// Runner coordinates trusted preparation, one agent session, verification, and cleanup.
type Runner struct {
	Registry  *Registry
	Preparer  ProjectPreparer
	Backend   ExecutionBackend
	Agent     EvaluationAgent
	Guidance  GuidanceResolver
	Artifacts *ArtifactStore
	Now       func() time.Time
}

// AttemptRequest contains invocation policy that does not belong in evaluation YAML.
type AttemptRequest struct {
	AttemptID       string
	LogicalTrialID  string
	Intent          RunIntent
	Definition      EvaluationDefinition
	Preparation     PreparationRequest
	GuidanceProfile string
	Runtime         RuntimeIdentity
}

// Run executes one logical attempt while preserving agent outcome and evaluator failures separately.
func (runner Runner) Run(ctx context.Context, request AttemptRequest) (result AttemptResult, runErr error) {
	if ctx == nil {
		ctx = context.Background()
	}
	now := runner.Now
	if now == nil {
		now = time.Now
	}
	result = AttemptResult{
		AttemptID:        request.AttemptID,
		LogicalTrialID:   request.LogicalTrialID,
		EvaluationID:     request.Definition.ID,
		PromptDigest:     request.Definition.PromptDigest,
		GuidanceProfile:  request.GuidanceProfile,
		Runtime:          request.Runtime,
		ScenarioID:       request.Definition.ProjectScenario,
		AgentOutcome:     AgentNotStarted,
		EvaluationStatus: EvaluationNotEvaluated,
		StartedAt:        now().UTC(),
	}
	var artifacts *AttemptArtifacts
	var planDigest string
	var baselineTree string
	var finalTree string
	var baselineDiff projectDiffSnapshot
	var supervisorEvents []Event
	var artifactEvents []Event
	var agentCapabilities AgentCapabilities
	var backendCapabilities []Capability
	var err error
	defer func() {
		result.FinishedAt = now().UTC()
		if artifacts == nil {
			return
		}
		normalizedEvents := normalizeArtifactEvents(artifactEvents)
		recordAttemptEvents(&result, artifacts, normalizedEvents)
		failures := writeAttemptReportArtifacts(artifacts, request, result, normalizedEvents, agentCapabilities.Capabilities, backendCapabilities)
		result.SecondaryFailures = append(result.SecondaryFailures, failures...)
		if len(failures) > 0 {
			result.EvaluationStatus = EvaluationEvaluatorError
		}
		if err := artifacts.WriteJSON("run.json", result); err != nil {
			result.SecondaryFailures = append(result.SecondaryFailures, SecondaryFailure{Phase: "artifact_run", Message: err.Error()})
			result.EvaluationStatus = EvaluationEvaluatorError
		}
		if result.Verification != nil {
			if err := artifacts.WriteJSON("verification.json", result.Verification); err != nil {
				result.SecondaryFailures = append(result.SecondaryFailures, SecondaryFailure{Phase: "artifact_verification", Message: err.Error()})
				result.EvaluationStatus = EvaluationEvaluatorError
			}
		}
		if attemptNeedsTriage(result) {
			triage := TriageRecord{State: TriageUnreviewed}
			if err := artifacts.WriteJSON("triage.json", triage); err != nil {
				result.SecondaryFailures = append(result.SecondaryFailures, SecondaryFailure{Phase: "artifact_triage", Message: err.Error()})
				result.EvaluationStatus = EvaluationEvaluatorError
			}
		}
		if _, err := artifacts.Finalize(planDigest, baselineTree, finalTree); err != nil {
			result.SecondaryFailures = append(result.SecondaryFailures, SecondaryFailure{Phase: "artifact_manifest", Message: err.Error()})
			result.EvaluationStatus = EvaluationEvaluatorError
		}
	}()

	if err := runner.validate(); err != nil {
		return result, err
	}
	switch request.Intent {
	case IntentAuthoritative, IntentDiagnostic:
	default:
		result.EvaluationStatus = EvaluationEvaluatorError
		return result, fmt.Errorf("evaluation intent %q is invalid", request.Intent)
	}
	result.Backend = runner.Backend.Name()
	result.Agent = runner.Agent.Name()
	if pathsOverlap(runner.Artifacts.root, request.Preparation.DestinationRoot) {
		return result, fmt.Errorf("artifact root and Project destination must not overlap")
	}
	if request.Preparation.ScenarioID != request.Definition.ProjectScenario {
		return result, fmt.Errorf("preparation scenario %q does not match evaluation scenario %q", request.Preparation.ScenarioID, request.Definition.ProjectScenario)
	}
	artifacts, err = runner.Artifacts.Begin(request.AttemptID)
	if err != nil {
		result.EvaluationStatus = EvaluationEvaluatorError
		return result, fmt.Errorf("begin attempt artifacts: %w", err)
	}
	resolved, err := runner.Registry.Resolve(request.Definition)
	if err != nil {
		return result, err
	}
	preparationCapabilities, err := runner.Preparer.Capabilities(ctx)
	if err != nil {
		return result, fmt.Errorf("preparation capabilities: %w", err)
	}
	agentCapabilities, err = runner.Agent.Capabilities(ctx)
	if err != nil {
		return result, fmt.Errorf("agent capabilities: %w", err)
	}
	backendCapabilities, err = runner.Backend.Capabilities(ctx)
	if err != nil {
		return result, fmt.Errorf("backend capabilities: %w", err)
	}
	plan, err := runner.Preparer.Resolve(ctx, request.Preparation)
	if err != nil {
		result.EvaluationStatus = EvaluationFixtureError
		return result, fmt.Errorf("resolve preparation: %w", err)
	}
	if err := validateResolvedPreparation(request.Definition, request.Preparation, plan); err != nil {
		result.EvaluationStatus = EvaluationFixtureError
		return result, err
	}
	planDigest = plan.PlanDigest
	result.PlanDigest = plan.PlanDigest
	result.ScenarioSchema = plan.ScenarioSchema
	result.ScenarioPlanDigest = plan.ScenarioPlanDigest
	result.CatalogDigest = plan.CatalogDigest
	result.DependencyDigests = cloneStringMap(plan.DependencyDigests)
	result.ProjectConfigDigest = digestBytes(plan.ProjectConfiguration)
	result.EnvironmentDigest = plan.EnvironmentDigest
	if !containsScenarioSchema(preparationCapabilities.ScenarioSchemaVersions, plan.ScenarioSchema) {
		result.EvaluationStatus = EvaluationIneligible
		result.Milestones = append(result.Milestones, MilestonePreflight, MilestoneEvaluationTerminal)
		return result, nil
	}
	// Only the backend can attest to candidate actions. Adapter capabilities describe
	// telemetry support and must never promote provider-originated events to evidence.
	available := effectiveBackendCapabilities(backendCapabilities, agentCapabilities.Capabilities)
	requiredCapabilities := append([]Capability(nil), resolved.Capabilities...)
	requiredCapabilities = append(requiredCapabilities, authoritativeCapabilities...)
	missing := missingCapabilities(requiredCapabilities, available)
	result.UnavailableEvidence = append([]Capability(nil), missing...)
	if len(missing) > 0 && request.Intent != IntentDiagnostic {
		result.EvaluationStatus = EvaluationIneligible
		result.UnavailableEvidence = missing
		result.Milestones = append(result.Milestones, MilestonePreflight, MilestoneEvaluationTerminal)
		return result, nil
	}
	result.Milestones = append(result.Milestones, MilestonePreflight)
	preparedProject, err := runner.Preparer.Prepare(ctx, request.Preparation, plan)
	if err != nil {
		result.EvaluationStatus = EvaluationFixtureError
		if preparedProject != nil {
			result.SecondaryFailures = append(result.SecondaryFailures, closeEvaluationResource("prepared_project", preparedProject.Close)...)
		}
		return result, fmt.Errorf("prepare Project: %w", err)
	}
	if preparedProject == nil {
		result.EvaluationStatus = EvaluationFixtureError
		return result, fmt.Errorf("preparer returned a nil Project")
	}
	defer func() {
		failures := closeEvaluationResource("prepared_project", preparedProject.Close)
		result.SecondaryFailures = append(result.SecondaryFailures, failures...)
		if len(failures) > 0 {
			result.EvaluationStatus = EvaluationEvaluatorError
		}
	}()
	if err := validatePreparationResult(plan, preparedProject.Result()); err != nil {
		result.EvaluationStatus = EvaluationFixtureError
		return result, err
	}
	_, preparedTree, err := snapshotProjectForDiff(preparedProject.Result().ProjectRoot)
	if err != nil {
		result.EvaluationStatus = EvaluationFixtureError
		return result, fmt.Errorf("snapshot prepared Project: %w", err)
	}
	if preparedTree != preparedProject.Result().BaselineTree {
		result.EvaluationStatus = EvaluationFixtureError
		return result, fmt.Errorf("prepared Project tree changed or uses an incompatible digest: got %s, want %s", preparedTree, preparedProject.Result().BaselineTree)
	}
	result.PreparedTree = preparedTree
	result.ForjExecutable = preparedProject.Result().ForjExecutable
	result.ForjDigest = preparedProject.Result().ForjDigest
	resolvedGuidance, err := runner.Guidance.Resolve(ctx, request.GuidanceProfile, preparedProject.Result())
	if err != nil {
		result.EvaluationStatus = EvaluationFixtureError
		return result, fmt.Errorf("resolve guidance: %w", err)
	}
	if resolvedGuidance.Profile != request.GuidanceProfile {
		result.EvaluationStatus = EvaluationFixtureError
		return result, fmt.Errorf("resolved guidance profile %q does not match requested profile %q", resolvedGuidance.Profile, request.GuidanceProfile)
	}
	result.GuidanceDigest = digestGuidance(resolvedGuidance)
	result.GuidanceFiles = guidanceFileIdentities(resolvedGuidance.Files)

	backendEnvironment, err := runner.Backend.Open(ctx, BackendRequest{
		Project:      preparedProject,
		ShellNetwork: request.Definition.Limits.ShellNetwork,
		Environment:  append([]string(nil), request.Preparation.Environment...),
		CommandLimit: request.Definition.Limits.Commands,
	})
	if err != nil {
		result.EvaluationStatus = EvaluationEvaluatorError
		if backendEnvironment != nil {
			result.SecondaryFailures = append(result.SecondaryFailures, closeEvaluationResource("backend", backendEnvironment.Close)...)
		}
		return result, fmt.Errorf("open execution backend: %w", err)
	}
	if backendEnvironment == nil {
		return result, fmt.Errorf("backend returned a nil environment")
	}
	defer func() {
		failures := closeEvaluationResource("backend", backendEnvironment.Close)
		result.SecondaryFailures = append(result.SecondaryFailures, failures...)
		if len(failures) > 0 {
			result.EvaluationStatus = EvaluationEvaluatorError
		}
	}()

	agentPreparation, err := runner.Agent.Prepare(ctx, backendEnvironment.Environment(), resolvedGuidance)
	if err != nil {
		result.AgentOutcome = AgentAdapterError
		if agentPreparation != nil {
			result.SecondaryFailures = append(result.SecondaryFailures, closeEvaluationResource("agent_preparation", agentPreparation.Close)...)
		}
		return result, fmt.Errorf("prepare agent: %w", err)
	}
	if agentPreparation == nil {
		result.AgentOutcome = AgentAdapterError
		return result, fmt.Errorf("agent returned a nil preparation")
	}
	defer func() {
		failures := closeEvaluationResource("agent_preparation", agentPreparation.Close)
		result.SecondaryFailures = append(result.SecondaryFailures, failures...)
		if len(failures) > 0 {
			result.EvaluationStatus = EvaluationEvaluatorError
		}
	}()
	preparedAgent := agentPreparation.Agent()
	result.Agent = preparedAgent.Name
	result.AgentExecutable = preparedAgent.Executable
	result.AgentDigest = preparedAgent.ExecutableDigest
	result.Model = preparedAgent.Model
	baselineDiff, baselineTree, err = snapshotProjectForDiff(preparedProject.Result().ProjectRoot)
	if err != nil {
		result.EvaluationStatus = EvaluationEvaluatorError
		return result, fmt.Errorf("snapshot treatment baseline Project: %w", err)
	}
	baseline, err := backendEnvironment.Baseline(ctx)
	if err != nil {
		result.EvaluationStatus = EvaluationEvaluatorError
		return result, fmt.Errorf("capture supervisor baseline: %w", err)
	}
	if request.Intent == IntentAuthoritative && (!baseline.Complete || baseline.TreeDigest == "") {
		result.EvaluationStatus = EvaluationIneligible
		result.Milestones = append(result.Milestones, MilestoneEvaluationTerminal)
		return result, nil
	}
	if baseline.TreeDigest != "" && baseline.TreeDigest != baselineTree {
		result.EvaluationStatus = EvaluationEvaluatorError
		return result, fmt.Errorf("supervisor baseline digest does not match Project snapshot: got %s, want %s", baseline.TreeDigest, baselineTree)
	}
	result.BaselineTree = baselineTree

	runContext, cancel := context.WithTimeout(ctx, request.Definition.Limits.WallTime)
	defer cancel()
	session, err := runner.Agent.Start(runContext, preparedAgent)
	if err != nil {
		result.AgentOutcome = classifyOperationError(runContext, err, AgentAdapterError)
		if session != nil {
			result.SecondaryFailures = append(result.SecondaryFailures, closeEvaluationResource("agent_session", session.Close)...)
		}
		return result, fmt.Errorf("start agent: %w", err)
	}
	if session == nil {
		result.AgentOutcome = AgentAdapterError
		return result, fmt.Errorf("agent returned a nil session")
	}
	sessionIdentity := session.Identity()
	result.AgentVersion = sessionIdentity.Version
	if sessionIdentity.Model != "" {
		result.Model = sessionIdentity.Model
	}
	result.ModelProvider = sessionIdentity.ModelProvider
	result.Milestones = append(result.Milestones, MilestoneProviderSessionStarted)
	sessionClosed := false
	defer func() {
		if sessionClosed {
			return
		}
		failures := closeEvaluationResource("agent_session", session.Close)
		result.SecondaryFailures = append(result.SecondaryFailures, failures...)
		if len(failures) > 0 {
			result.EvaluationStatus = EvaluationEvaluatorError
		}
	}()

	turn, err := session.Turn(runContext, AgentTurn{Prompt: request.Definition.Prompt, Limits: request.Definition.Limits})
	artifactEvents = appendAdapterEvents(artifactEvents, turn.Events)
	if err != nil {
		result.AgentOutcome = classifyOperationError(runContext, err, AgentAdapterError)
		return result, fmt.Errorf("deliver prompt: %w", err)
	}
	if !turn.Accepted {
		result.AgentOutcome = AgentAdapterError
		return result, fmt.Errorf("agent did not accept the prompt")
	}
	result.Milestones = append(result.Milestones, MilestonePromptDelivered)
	agentResult, err := session.Wait(runContext)
	artifactEvents = appendAdapterEvents(artifactEvents, agentResult.Events)
	if err != nil {
		result.AgentOutcome = classifyOperationError(runContext, err, AgentProviderError)
		observed, observationErr := backendEnvironment.ObservedEvents(ctx)
		if observationErr != nil {
			result.SecondaryFailures = append(result.SecondaryFailures, SecondaryFailure{Phase: "event_capture", Message: observationErr.Error()})
		} else if validationErr := validateSupervisorEvents(observed, requiresSupervisorTerminal(backendCapabilities)); validationErr != nil {
			result.SecondaryFailures = append(result.SecondaryFailures, SecondaryFailure{Phase: "event_capture", Message: validationErr.Error()})
			result.EvaluationStatus = EvaluationEvaluatorError
		} else {
			supervisorEvents = observed
			artifactEvents = appendSupervisorEvents(artifactEvents, observed)
			if len(supervisorEvents) > 0 && !containsMilestone(result.Milestones, MilestoneFirstAgentAction) {
				result.Milestones = append(result.Milestones, MilestoneFirstAgentAction)
			}
		}
		if containsMilestone(result.Milestones, MilestoneFirstAgentAction) {
			result.EvaluationStatus = EvaluationEvaluatorError
		}
		return result, fmt.Errorf("wait for agent: %w", err)
	}
	supervisorEvents, err = backendEnvironment.ObservedEvents(ctx)
	if err != nil {
		result.EvaluationStatus = EvaluationEvaluatorError
		return result, fmt.Errorf("collect supervisor events: %w", err)
	}
	if err := validateSupervisorEvents(supervisorEvents, requiresSupervisorTerminal(backendCapabilities)); err != nil {
		result.EvaluationStatus = EvaluationEvaluatorError
		return result, err
	}
	artifactEvents = appendSupervisorEvents(artifactEvents, supervisorEvents)
	if countCommandEvents(supervisorEvents) > request.Definition.Limits.Commands {
		result.AgentOutcome = AgentAdapterError
		result.EvaluationStatus = EvaluationEvaluatorError
		return result, fmt.Errorf("agent exceeded command limit %d", request.Definition.Limits.Commands)
	}
	if len(supervisorEvents) > 0 && !containsMilestone(result.Milestones, MilestoneFirstAgentAction) {
		result.Milestones = append(result.Milestones, MilestoneFirstAgentAction)
	}
	result.AgentOutcome = agentResult.Outcome
	if result.AgentOutcome == "" {
		result.AgentOutcome = AgentCompleted
	}
	result.Milestones = append(result.Milestones, MilestoneAgentTerminal)
	if failures := closeEvaluationResource("agent_session", session.Close); len(failures) > 0 {
		result.SecondaryFailures = append(result.SecondaryFailures, failures...)
		result.EvaluationStatus = EvaluationEvaluatorError
		return result, fmt.Errorf("agent session cleanup failed before verification")
	}
	sessionClosed = true
	sealed, err := backendEnvironment.Seal(ctx)
	if err != nil {
		result.EvaluationStatus = EvaluationEvaluatorError
		return result, fmt.Errorf("seal candidate Project: %w", err)
	}
	if strings.TrimSpace(sealed.Root) == "" || strings.TrimSpace(sealed.TreeDigest) == "" {
		result.EvaluationStatus = EvaluationEvaluatorError
		return result, fmt.Errorf("backend returned an incomplete sealed Project")
	}
	finalTree = sealed.TreeDigest
	result.FinalTree = finalTree
	diff, diffErr := buildProjectDiff(baselineDiff, sealed.Root)
	if diffErr != nil {
		result.SecondaryFailures = append(result.SecondaryFailures, SecondaryFailure{Phase: "artifact_diff", Message: diffErr.Error()})
		result.EvaluationStatus = EvaluationEvaluatorError
	} else if err := artifacts.WriteText("diff.patch", diff); err != nil {
		result.SecondaryFailures = append(result.SecondaryFailures, SecondaryFailure{Phase: "artifact_diff", Message: err.Error()})
		result.EvaluationStatus = EvaluationEvaluatorError
	}

	changes, err := projectChanges(baselineDiff, sealed.Root)
	if err != nil {
		result.EvaluationStatus = EvaluationEvaluatorError
		return result, fmt.Errorf("project change projection: %w", err)
	}
	verification, err := resolved.Verifier.Verify(ctx, VerificationInput{
		ProjectRoot:  sealed.Root,
		BaselineTree: baselineTree,
		FinalTree:    sealed.TreeDigest,
		Changes:      changes,
		Events:       supervisorEvents,
	})
	if err != nil {
		result.EvaluationStatus = EvaluationEvaluatorError
		return result, fmt.Errorf("verify evaluation: %w", err)
	}
	workflow, workflowChecks := EvaluateWorkflow(resolved.Workflow, supervisorEvents, preparedProject.Result().ForjDigest, available)
	verification.WorkflowConformance = workflow
	verification.Checks = append(verification.Checks, workflowChecks...)
	result.Verification = &verification
	if request.Intent == IntentDiagnostic {
		verification.FrameworkOutcome = EndpointResult{ID: verification.FrameworkOutcome.ID, Status: EndpointIneligible, Details: "diagnostic backend cannot establish an authoritative framework outcome"}
		if verification.Contract != nil {
			verification.Contract = &EndpointResult{ID: verification.Contract.ID, Status: EndpointIneligible, Details: "diagnostic backend cannot establish an authoritative contract outcome"}
		}
		result.Verification = &verification
		result.EvaluationStatus = EvaluationDiagnostic
	} else if result.EvaluationStatus != EvaluationEvaluatorError {
		if result.AgentOutcome == AgentAbstained {
			result.EvaluationStatus = EvaluationValidAbstention
		} else {
			result.EvaluationStatus = EvaluationValid
		}
	}
	result.Milestones = append(result.Milestones, MilestoneEvaluationTerminal)
	return result, nil
}

// attemptNeedsTriage keeps valid evidence distinct from a successful application or workflow result.
func attemptNeedsTriage(result AttemptResult) bool {
	if result.EvaluationStatus != EvaluationValid && result.EvaluationStatus != EvaluationValidAbstention {
		return true
	}
	if result.Verification == nil {
		return false
	}
	if result.Verification.FrameworkOutcome.Status == EndpointFailed || result.Verification.WorkflowConformance.Status == EndpointFailed {
		return true
	}
	if result.Verification.Contract != nil && result.Verification.Contract.Status == EndpointFailed {
		return true
	}
	for _, check := range result.Verification.Checks {
		if check.Status == EndpointFailed {
			return true
		}
	}
	return false
}

// cloneStringMap retains resolved identities without exposing preparer-owned mutable maps to later phases.
func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

// digestBytes records immutable configuration identity without retaining potentially sensitive source text.
func digestBytes(body []byte) string {
	digest := sha256.Sum256(body)
	return fmt.Sprintf("sha256:%x", digest)
}

// guidanceFileIdentities records a stable per-file treatment manifest beside the aggregate guidance digest.
func guidanceFileIdentities(files map[string][]byte) []GuidanceFileIdentity {
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	identities := make([]GuidanceFileIdentity, 0, len(paths))
	for _, path := range paths {
		digest := sha256.Sum256(files[path])
		identities = append(identities, GuidanceFileIdentity{Path: path, Digest: fmt.Sprintf("sha256:%x", digest)})
	}
	return identities
}

// digestGuidance binds the selected profile and every native projection input without retaining instruction content in run metadata.
func digestGuidance(guidance Guidance) string {
	hash := sha256.New()
	fmt.Fprintf(hash, "profile\x00%s\x00", guidance.Profile)
	paths := make([]string, 0, len(guidance.Files))
	for path := range guidance.Files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		fmt.Fprintf(hash, "file\x00%s\x00", path)
		hash.Write(guidance.Files[path])
		hash.Write([]byte{0})
	}
	for _, group := range [][]string{append([]string(nil), guidance.Skills...), append([]string(nil), guidance.MCP...)} {
		sort.Strings(group)
		for _, value := range group {
			fmt.Fprintf(hash, "%s\x00", value)
		}
		hash.Write([]byte{0})
	}
	return fmt.Sprintf("sha256:%x", hash.Sum(nil))
}

// countCommandEvents verifies the adapter honored its command budget without double-counting completion events.
func countCommandEvents(events []Event) int {
	count := 0
	for _, event := range events {
		if event.Kind == EventCommandStarted {
			count++
		}
	}
	return count
}

// pathsOverlap rejects evidence roots that an agent could reach through its writable Project tree.
func pathsOverlap(left, right string) bool {
	if strings.TrimSpace(left) == "" || strings.TrimSpace(right) == "" {
		return false
	}
	left, leftErr := filepath.Abs(left)
	right, rightErr := filepath.Abs(right)
	if leftErr != nil || rightErr != nil {
		return true
	}
	within := func(parent, child string) bool {
		relative, err := filepath.Rel(parent, child)
		return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
	}
	return within(left, right) || within(right, left)
}

// containsScenarioSchema reports whether a preparer explicitly negotiated the resolved schema.
func containsScenarioSchema(versions []int, target int) bool {
	for _, version := range versions {
		if version == target {
			return true
		}
	}
	return false
}

// validate checks immutable runner collaborators before a trial acquires resources.
func (runner Runner) validate() error {
	if runner.Registry == nil || runner.Preparer == nil || runner.Backend == nil || runner.Agent == nil || runner.Guidance == nil || runner.Artifacts == nil {
		return fmt.Errorf("evaluation runner requires a registry, preparer, backend, agent, guidance resolver, and artifact store")
	}
	return nil
}

// recordAttemptEvents persists redacted evidence while retaining capture failures beside the attempt outcome.
func recordAttemptEvents(result *AttemptResult, artifacts *AttemptArtifacts, events []Event) {
	for _, event := range events {
		if err := artifacts.AppendEvent(event); err != nil {
			result.SecondaryFailures = append(result.SecondaryFailures, SecondaryFailure{Phase: "event_capture", Message: err.Error()})
			result.EvaluationStatus = EvaluationEvaluatorError
			return
		}
	}
}

// appendAdapterEvents retains provider telemetry for diagnostics without allowing it to impersonate supervisor evidence.
func appendAdapterEvents(destination []Event, events []Event) []Event {
	for _, event := range events {
		event.Source = EventSourceAdapter
		destination = append(destination, event)
	}
	return destination
}

// appendSupervisorEvents retains validated backend observations with their trusted source classification.
func appendSupervisorEvents(destination []Event, events []Event) []Event {
	for _, event := range events {
		event.Source = EventSourceSupervisor
		destination = append(destination, event)
	}
	return destination
}

// normalizeArtifactEvents produces one stable sequence while preserving each event's evidence boundary.
func normalizeArtifactEvents(events []Event) []Event {
	result := append([]Event(nil), events...)
	for index := range result {
		result[index].Sequence = uint64(index + 1)
	}
	return result
}

// validateResolvedPreparation prevents a preparer from swapping the requested scenario or exposing target work.
func validateResolvedPreparation(definition EvaluationDefinition, request PreparationRequest, plan ResolvedPreparationPlan) error {
	if plan.ScenarioID != definition.ProjectScenario {
		return fmt.Errorf("resolved scenario %q does not match requested scenario %q", plan.ScenarioID, definition.ProjectScenario)
	}
	if plan.ResolutionID != request.OrchestrationID {
		return fmt.Errorf("resolved preparation identity does not match the orchestration request")
	}
	if plan.ResolutionID == "" || plan.ScenarioSchema <= 0 || plan.PlanDigest == "" || plan.ScenarioPlanDigest == "" || plan.CatalogDigest == "" || plan.ForjDigest == "" || plan.EnvironmentDigest == "" {
		return fmt.Errorf("resolved preparation plan is incomplete")
	}
	if !plan.TargetOmitted {
		return fmt.Errorf("resolved preparation plan does not prove target omission")
	}
	return nil
}

// validatePreparationResult binds candidate execution back to the trusted resolved plan.
func validatePreparationResult(plan ResolvedPreparationPlan, result PreparationResult) error {
	if result.ResolutionID != plan.ResolutionID {
		return fmt.Errorf("prepared resolution identity does not match the resolved plan")
	}
	if result.ScenarioID != plan.ScenarioID || result.ScenarioSchema != plan.ScenarioSchema {
		return fmt.Errorf("prepared scenario identity does not match the resolved plan")
	}
	if result.PlanDigest != plan.PlanDigest || result.CatalogDigest != plan.CatalogDigest {
		return fmt.Errorf("prepared Project provenance does not match the resolved plan")
	}
	if result.ForjDigest != plan.ForjDigest {
		return fmt.Errorf("prepared GoForj executable does not match the resolved plan")
	}
	if result.ProjectRoot == "" || result.BaselineTree == "" || result.ForjExecutable == "" || result.ForjDigest == "" {
		return fmt.Errorf("preparation result is incomplete")
	}
	return nil
}

// intersectCapabilities retains only properties jointly supported by preparation, adapter, and backend.
func intersectCapabilities(groups ...[]Capability) []Capability {
	if len(groups) == 0 {
		return nil
	}
	counts := map[Capability]int{}
	for _, group := range groups {
		seen := map[Capability]bool{}
		for _, capability := range group {
			if capability != "" && !seen[capability] {
				counts[capability]++
				seen[capability] = true
			}
		}
	}
	var intersection []Capability
	for capability, count := range counts {
		if count == len(groups) {
			intersection = append(intersection, capability)
		}
	}
	sort.Slice(intersection, func(left, right int) bool { return intersection[left] < intersection[right] })
	return intersection
}

// effectiveBackendCapabilities keeps backend observation authoritative while requiring the adapter to prove properties it can weaken.
func effectiveBackendCapabilities(backendCapabilities, agentCapabilities []Capability) []Capability {
	available := intersectCapabilities(backendCapabilities)
	if capabilityAvailable(agentCapabilities, CapabilityCredentialIsolation) {
		return available
	}
	filtered := available[:0]
	for _, capability := range available {
		if capability != CapabilityCredentialIsolation {
			filtered = append(filtered, capability)
		}
	}
	return filtered
}

// capabilityAvailable reports whether one component explicitly claims a capability.
func capabilityAvailable(capabilities []Capability, target Capability) bool {
	for _, capability := range capabilities {
		if capability == target {
			return true
		}
	}
	return false
}

// missingCapabilities returns the sorted evidence classes unavailable during preflight.
func missingCapabilities(required, available []Capability) []Capability {
	availableSet := map[Capability]bool{}
	for _, capability := range available {
		availableSet[capability] = true
	}
	var missing []Capability
	for _, capability := range required {
		if !availableSet[capability] {
			missing = append(missing, capability)
		}
	}
	sort.Slice(missing, func(left, right int) bool { return missing[left] < missing[right] })
	return missing
}

// classifyOperationError preserves a typed provider/adapter failure unless the run context supplies a stronger terminal cause.
func classifyOperationError(ctx context.Context, err error, fallback AgentOutcome) AgentOutcome {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return AgentTimeout
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		return AgentCancelled
	}
	var failure *AgentFailure
	if errors.As(err, &failure) && (failure.Outcome == AgentProviderError || failure.Outcome == AgentAdapterError) {
		return failure.Outcome
	}
	return fallback
}

// validateSupervisorEvents rejects ambiguous or adapter-controlled records before workflow correlation.
func validateSupervisorEvents(events []Event, requireTerminal bool) error {
	var lastSequence uint64
	started := make(map[string]bool)
	finished := make(map[string]bool)
	terminalEvents := 0
	for _, event := range events {
		if event.Source != EventSourceSupervisor {
			return fmt.Errorf("event %d does not have supervisor provenance", event.Sequence)
		}
		if event.Sequence == 0 || event.Sequence <= lastSequence {
			return fmt.Errorf("supervisor event sequence %d is not strictly increasing", event.Sequence)
		}
		lastSequence = event.Sequence
		switch event.Kind {
		case EventCommandStarted:
			commandID := strings.TrimSpace(event.Fields[EventFieldCommandID])
			if commandID == "" || started[commandID] {
				return fmt.Errorf("supervisor command start %d has an empty or duplicate command ID", event.Sequence)
			}
			if strings.TrimSpace(event.Fields[EventFieldExecutableDigest]) == "" {
				return fmt.Errorf("supervisor command start %q has no executable digest", commandID)
			}
			var arguments []string
			if err := json.Unmarshal([]byte(event.Fields[EventFieldArguments]), &arguments); err != nil {
				return fmt.Errorf("supervisor command start %q has invalid arguments: %w", commandID, err)
			}
			started[commandID] = true
		case EventCommandFinished:
			commandID := strings.TrimSpace(event.Fields[EventFieldCommandID])
			if commandID == "" || !started[commandID] || finished[commandID] {
				return fmt.Errorf("supervisor command finish %d has an unmatched or duplicate command ID", event.Sequence)
			}
			if _, err := strconv.Atoi(event.Fields[EventFieldExitCode]); err != nil {
				return fmt.Errorf("supervisor command finish %q has an invalid exit code", commandID)
			}
			finished[commandID] = true
		case EventRunFinished:
			terminalEvents++
			if terminalEvents > 1 || event.Sequence != events[len(events)-1].Sequence {
				return fmt.Errorf("supervisor run terminal must appear exactly once at the end")
			}
		}
	}
	for commandID := range started {
		if !finished[commandID] {
			return fmt.Errorf("supervisor command %q has no completion", commandID)
		}
	}
	if requireTerminal && terminalEvents != 1 {
		return fmt.Errorf("supervisor event stream has no unique terminal marker")
	}
	return nil
}

// requiresSupervisorTerminal identifies capabilities whose observations need an explicit complete-stream marker.
func requiresSupervisorTerminal(capabilities []Capability) bool {
	for _, capability := range capabilities {
		switch capability {
		case CapabilityCommands, CapabilityFileReads, CapabilityFileWrites, CapabilityMCPToolCalls:
			return true
		}
	}
	return false
}

// closeEvaluationResource gives teardown a fresh budget and preserves every secondary failure.
func closeEvaluationResource(phase string, close func(context.Context) error) []SecondaryFailure {
	cleanupContext, cancel := context.WithTimeout(context.Background(), evaluationCleanupTimeout)
	defer cancel()
	if err := close(cleanupContext); err != nil {
		return []SecondaryFailure{{Phase: phase, Message: err.Error()}}
	}
	return nil
}

// containsMilestone prevents duplicate lifecycle boundaries when events arrive in multiple adapter phases.
func containsMilestone(milestones []Milestone, target Milestone) bool {
	for _, milestone := range milestones {
		if milestone == target {
			return true
		}
	}
	return false
}
