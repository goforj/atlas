package eval

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
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
	var baselineTests []TrustedTestFile
	var supervisorEvents []Event
	var artifactEvents []Event
	var agentProperties AgentProperties
	var backendCapabilities []Capability
	var promptDelivered bool
	var sessionClosed bool
	var err error
	defer func() {
		result.FinishedAt = now().UTC()
		if err := finalizeAttemptArtifacts(artifacts, request, &result, artifactEvents, agentProperties.Properties, backendCapabilities, planDigest, baselineTree, finalTree); err != nil {
			runErr = errors.Join(runErr, fmt.Errorf("finalize attempt artifacts: %w", err))
		}
		if runErr == nil && result.EvaluationStatus == EvaluationEvaluatorError {
			runErr = fmt.Errorf("evaluation integrity failed during deferred cleanup or artifact finalization")
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
	overlap, err := pathsOverlap(runner.Artifacts.root, request.Preparation.DestinationRoot)
	if err != nil {
		return result, fmt.Errorf("resolve artifact and Project roots: %w", err)
	}
	if overlap {
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
	preflight, terminal, err := runner.preflightAttempt(ctx, request, &result)
	if err != nil || terminal {
		return result, err
	}
	resolved := preflight.resolved
	plan := preflight.plan
	agentProperties = preflight.agentProperties
	backendCapabilities = preflight.backendCapabilities
	available := preflight.availableCapabilities
	planDigest = plan.PlanDigest
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
	resolvedGuidance, err = runner.Preparer.MaterializeGuidance(ctx, preparedProject, resolvedGuidance)
	if err != nil {
		result.EvaluationStatus = EvaluationFixtureError
		return result, fmt.Errorf("materialize guidance: %w", err)
	}
	if resolvedGuidance.Profile != request.GuidanceProfile {
		result.EvaluationStatus = EvaluationFixtureError
		return result, fmt.Errorf("materialized guidance profile %q does not match requested profile %q", resolvedGuidance.Profile, request.GuidanceProfile)
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
	defer func() {
		if !promptDelivered || finalTree != "" {
			return
		}
		if !sessionClosed {
			result.SecondaryFailures = append(result.SecondaryFailures, SecondaryFailure{Phase: "final_state", Message: "candidate state was not retained because the agent session did not stop cleanly"})
			result.EvaluationStatus = EvaluationEvaluatorError
			return
		}
		retainPostPromptFinalState(backendEnvironment, baselineDiff, artifacts, &result, &finalTree)
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
	result.ProviderAuthorityDigest = preparedAgent.AuthorityDigest
	result.Model = preparedAgent.Model
	baselineDiff, baselineTree, err = snapshotProjectForDiff(preparedProject.Result().ProjectRoot)
	if err != nil {
		result.EvaluationStatus = EvaluationEvaluatorError
		return result, fmt.Errorf("snapshot treatment baseline Project: %w", err)
	}
	baselineTests, err = trustedTestsFromSnapshot(baselineDiff)
	if err != nil {
		result.EvaluationStatus = EvaluationFixtureError
		return result, fmt.Errorf("capture trusted baseline tests: %w", err)
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
	if sessionIdentity.AuthorityDigest != "" {
		result.ProviderAuthorityDigest = sessionIdentity.AuthorityDigest
	}
	result.ProviderSessionDigest = sessionIdentity.SessionDigest
	result.Milestones = append(result.Milestones, MilestoneProviderSessionStarted)
	defer func() {
		if sessionClosed {
			return
		}
		failures := closeEvaluationResource("agent_session", session.Close)
		result.SecondaryFailures = append(result.SecondaryFailures, failures...)
		if len(failures) > 0 {
			result.EvaluationStatus = EvaluationEvaluatorError
			return
		}
		sessionClosed = true
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
	promptDelivered = true
	result.Milestones = append(result.Milestones, MilestonePromptDelivered)
	agentResult, err := session.Wait(runContext)
	artifactEvents = appendAdapterEvents(artifactEvents, agentResult.Events)
	result.ProviderTelemetry = cloneProviderTelemetry(agentResult.Telemetry)
	if err == nil && agentResult.Outcome != "" && agentResult.Outcome != AgentCompleted {
		err = &AgentFailure{
			Outcome: agentResult.Outcome,
			Err:     fmt.Errorf("agent reached terminal outcome %q", agentResult.Outcome),
		}
	}
	if request.Intent == IntentDiagnostic && adapterCommandCount(turn.Events, agentResult) > uint64(request.Definition.Limits.Commands) {
		result.AgentOutcome = AgentAdapterError
		result.EvaluationStatus = EvaluationDiagnostic
		return result, fmt.Errorf("adapter telemetry exceeded command limit %d", request.Definition.Limits.Commands)
	}
	if err != nil {
		result.AgentOutcome = classifyOperationError(runContext, err, AgentProviderError)
		observed, observationErr := backendEnvironment.ObservedEvents(ctx)
		if observationErr != nil {
			result.SecondaryFailures = append(result.SecondaryFailures, SecondaryFailure{Phase: "event_capture", Message: observationErr.Error(), Cause: observationErr})
		} else if validationErr := validateSupervisorEvents(observed, requiresSupervisorTerminal(backendCapabilities)); validationErr != nil {
			result.SecondaryFailures = append(result.SecondaryFailures, SecondaryFailure{Phase: "event_capture", Message: validationErr.Error()})
			result.EvaluationStatus = EvaluationEvaluatorError
		} else {
			supervisorEvents = observed
			artifactEvents = appendSupervisorEvents(artifactEvents, observed)
			if firstAgentActionObserved(supervisorEvents, agentResult.Message, available) && !containsMilestone(result.Milestones, MilestoneFirstAgentAction) {
				result.Milestones = append(result.Milestones, MilestoneFirstAgentAction)
			}
		}
		if containsMilestone(result.Milestones, MilestoneFirstAgentAction) {
			result.EvaluationStatus = EvaluationEvaluatorError
		}
		result.SecondaryFailures = append(result.SecondaryFailures, SecondaryFailure{Phase: "agent_terminal", Message: err.Error(), Cause: err})
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
	if firstAgentActionObserved(supervisorEvents, agentResult.Message, available) && !containsMilestone(result.Milestones, MilestoneFirstAgentAction) {
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
	if strings.TrimSpace(sealed.Root) != "" && strings.TrimSpace(sealed.TreeDigest) != "" {
		finalTree = sealed.TreeDigest
	}
	if err := verifySealedAttempt(ctx, sealed, baselineDiff, baselineTests, baselineTree, supervisorEvents, agentResult.Message, preparedProject.Result().ForjDigest, available, resolved, request.Intent, artifacts, &result); err != nil {
		return result, err
	}
	return result, nil
}

// retainPostPromptFinalState seals and records candidate state after a delivered prompt cannot complete normally.
func retainPostPromptFinalState(backend BackendEnvironment, baseline projectDiffSnapshot, artifacts *AttemptArtifacts, result *AttemptResult, finalTree *string) {
	cleanupContext, cancel := context.WithTimeout(context.Background(), evaluationCleanupTimeout)
	defer cancel()
	sealed, err := backend.Seal(cleanupContext)
	if err != nil {
		result.SecondaryFailures = append(result.SecondaryFailures, SecondaryFailure{Phase: "final_state", Message: err.Error(), Cause: err})
		result.EvaluationStatus = EvaluationEvaluatorError
		return
	}
	if strings.TrimSpace(sealed.Root) == "" || strings.TrimSpace(sealed.TreeDigest) == "" {
		result.SecondaryFailures = append(result.SecondaryFailures, SecondaryFailure{Phase: "final_state", Message: "backend returned an incomplete sealed Project"})
		result.EvaluationStatus = EvaluationEvaluatorError
		return
	}
	result.FinalTree = sealed.TreeDigest
	*finalTree = sealed.TreeDigest
	diff, err := buildProjectDiff(baseline, sealed.Root)
	if err != nil {
		result.SecondaryFailures = append(result.SecondaryFailures, SecondaryFailure{Phase: "artifact_diff", Message: err.Error(), Cause: err})
		result.EvaluationStatus = EvaluationEvaluatorError
		return
	}
	if err := artifacts.WriteText("diff.patch", diff); err != nil {
		result.SecondaryFailures = append(result.SecondaryFailures, SecondaryFailure{Phase: "artifact_diff", Message: err.Error(), Cause: err})
		result.EvaluationStatus = EvaluationEvaluatorError
	}
}

// attemptPreflight contains resolved identities needed after capability and fixture admission.
type attemptPreflight struct {
	resolved              ResolvedEvaluation
	plan                  ResolvedPreparationPlan
	agentProperties       AgentProperties
	backendCapabilities   []Capability
	availableCapabilities []Capability
}

// preflightAttempt resolves immutable inputs and stops ineligible attempts before Project mutation.
func (runner Runner) preflightAttempt(ctx context.Context, request AttemptRequest, result *AttemptResult) (attemptPreflight, bool, error) {
	resolved, err := runner.Registry.Resolve(request.Definition)
	if err != nil {
		return attemptPreflight{}, false, err
	}
	preparationCapabilities, err := runner.Preparer.Capabilities(ctx)
	if err != nil {
		return attemptPreflight{}, false, fmt.Errorf("preparation capabilities: %w", err)
	}
	agentProperties, err := runner.Agent.Properties(ctx)
	if err != nil {
		return attemptPreflight{}, false, fmt.Errorf("agent capabilities: %w", err)
	}
	backendCapabilities, err := runner.Backend.Capabilities(ctx)
	if err != nil {
		return attemptPreflight{}, false, fmt.Errorf("backend capabilities: %w", err)
	}
	plan, err := runner.Preparer.Resolve(ctx, request.Preparation)
	if err != nil {
		result.EvaluationStatus = EvaluationFixtureError
		return attemptPreflight{}, false, fmt.Errorf("resolve preparation: %w", err)
	}
	if err := validateResolvedPreparation(request.Definition, request.Preparation, plan); err != nil {
		result.EvaluationStatus = EvaluationFixtureError
		return attemptPreflight{}, false, err
	}
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
		return attemptPreflight{}, true, nil
	}
	// Only the backend can attest to candidate actions. Adapter capabilities describe
	// telemetry support and must never promote provider-originated events to evidence.
	available := effectiveBackendCapabilities(backendCapabilities, agentProperties.Properties)
	requiredCapabilities := append([]Capability(nil), resolved.Capabilities...)
	requiredCapabilities = append(requiredCapabilities, authoritativeCapabilities...)
	missing := missingCapabilities(requiredCapabilities, available)
	result.UnavailableEvidence = append([]Capability(nil), missing...)
	if len(missing) > 0 && request.Intent != IntentDiagnostic {
		result.EvaluationStatus = EvaluationIneligible
		result.Milestones = append(result.Milestones, MilestonePreflight, MilestoneEvaluationTerminal)
		return attemptPreflight{}, true, nil
	}
	result.Milestones = append(result.Milestones, MilestonePreflight)
	return attemptPreflight{
		resolved:              resolved,
		plan:                  plan,
		agentProperties:       agentProperties,
		backendCapabilities:   backendCapabilities,
		availableCapabilities: available,
	}, false, nil
}

// firstAgentActionObserved records supervisor evidence, or an adapter-proven exact terminal response.
func firstAgentActionObserved(events []Event, terminalResponse string, capabilities []Capability) bool {
	if len(events) > 0 {
		return true
	}
	return capabilityAvailable(capabilities, CapabilityFinalResponseCapture) && strings.TrimSpace(terminalResponse) != ""
}

// verifySealedAttempt projects the immutable candidate tree into outcome and workflow results.
func verifySealedAttempt(ctx context.Context, sealed SealedProject, baselineDiff projectDiffSnapshot, baselineTests []TrustedTestFile, baselineTree string, events []Event, finalResponse, forjDigest string, available []Capability, resolved ResolvedEvaluation, intent RunIntent, artifacts *AttemptArtifacts, result *AttemptResult) error {
	if strings.TrimSpace(sealed.Root) == "" || strings.TrimSpace(sealed.TreeDigest) == "" {
		result.EvaluationStatus = EvaluationEvaluatorError
		return fmt.Errorf("backend returned an incomplete sealed Project")
	}
	result.FinalTree = sealed.TreeDigest
	diff, diffErr := buildProjectDiff(baselineDiff, sealed.Root)
	if diffErr != nil {
		result.SecondaryFailures = append(result.SecondaryFailures, SecondaryFailure{Phase: "artifact_diff", Message: diffErr.Error(), Cause: diffErr})
		result.EvaluationStatus = EvaluationEvaluatorError
	} else if err := artifacts.WriteText("diff.patch", diff); err != nil {
		result.SecondaryFailures = append(result.SecondaryFailures, SecondaryFailure{Phase: "artifact_diff", Message: err.Error(), Cause: err})
		result.EvaluationStatus = EvaluationEvaluatorError
	}
	changes, err := projectChanges(baselineDiff, sealed.Root)
	if err != nil {
		result.EvaluationStatus = EvaluationEvaluatorError
		return fmt.Errorf("project change projection: %w", err)
	}
	verification, err := resolved.Verifier.Verify(ctx, VerificationInput{
		ProjectRoot:   sealed.Root,
		BaselineTree:  baselineTree,
		FinalTree:     sealed.TreeDigest,
		BaselineTests: baselineTests,
		Changes:       changes,
		Events:        events,
		FinalResponse: finalResponse,
	})
	if err != nil {
		result.EvaluationStatus = EvaluationEvaluatorError
		return fmt.Errorf("verify evaluation: %w", err)
	}
	workflow, workflowChecks := EvaluateWorkflow(resolved.Workflow, events, forjDigest, available)
	verification.WorkflowConformance = workflow
	verification.Checks = append(verification.Checks, workflowChecks...)
	result.Verification = &verification
	if verification.Abstention != nil && verification.Abstention.Status == EndpointPassed {
		result.AgentOutcome = AgentAbstained
	}
	if intent == IntentDiagnostic {
		verification.FrameworkOutcome = EndpointResult{ID: verification.FrameworkOutcome.ID, Status: EndpointIneligible, Details: "diagnostic backend cannot establish an authoritative framework outcome"}
		if verification.Contract != nil {
			verification.Contract = &EndpointResult{ID: verification.Contract.ID, Status: EndpointIneligible, Details: "diagnostic backend cannot establish an authoritative contract outcome"}
		}
		result.Verification = &verification
		if result.EvaluationStatus != EvaluationEvaluatorError {
			result.EvaluationStatus = EvaluationDiagnostic
		}
	} else if result.EvaluationStatus != EvaluationEvaluatorError {
		if result.AgentOutcome == AgentAbstained {
			result.EvaluationStatus = EvaluationValidAbstention
		} else {
			result.EvaluationStatus = EvaluationValid
		}
	}
	result.Milestones = append(result.Milestones, MilestoneEvaluationTerminal)
	return nil
}

// finalizeAttemptArtifacts keeps terminal evidence repair independent from the execution state machine.
func finalizeAttemptArtifacts(artifacts *AttemptArtifacts, request AttemptRequest, result *AttemptResult, events []Event, agentProperties []Capability, backendCapabilities []Capability, planDigest, baselineTree, finalTree string) error {
	if artifacts == nil {
		return nil
	}
	normalizedEvents := normalizeArtifactEvents(events)
	recordAttemptEvents(result, artifacts, normalizedEvents)
	failures := writeAttemptReportArtifacts(artifacts, request, *result, normalizedEvents, agentProperties, backendCapabilities)
	result.SecondaryFailures = append(result.SecondaryFailures, failures...)
	var finalizationErr error
	for _, failure := range result.SecondaryFailures {
		finalizationErr = errors.Join(finalizationErr, failure.Cause)
	}
	artifactWriteFailed := len(failures) > 0 || hasArtifactFailure(result.SecondaryFailures)
	if artifactWriteFailed {
		result.EvaluationStatus = EvaluationEvaluatorError
	}
	if err := artifacts.WriteJSON("run.json", result); err != nil {
		result.SecondaryFailures = append(result.SecondaryFailures, SecondaryFailure{Phase: "artifact_run", Message: err.Error(), Cause: err})
		finalizationErr = errors.Join(finalizationErr, err)
		result.EvaluationStatus = EvaluationEvaluatorError
		artifactWriteFailed = true
	}
	if result.Verification != nil {
		if err := artifacts.WriteJSON("verification.json", result.Verification); err != nil {
			result.SecondaryFailures = append(result.SecondaryFailures, SecondaryFailure{Phase: "artifact_verification", Message: err.Error(), Cause: err})
			finalizationErr = errors.Join(finalizationErr, err)
			result.EvaluationStatus = EvaluationEvaluatorError
			artifactWriteFailed = true
		}
	}
	if attemptNeedsTriage(*result) {
		if err := artifacts.WriteJSON("triage.json", TriageRecord{State: TriageUnreviewed}); err != nil {
			result.SecondaryFailures = append(result.SecondaryFailures, SecondaryFailure{Phase: "artifact_triage", Message: err.Error(), Cause: err})
			finalizationErr = errors.Join(finalizationErr, err)
			result.EvaluationStatus = EvaluationEvaluatorError
			artifactWriteFailed = true
		}
	}
	if artifactWriteFailed {
		if err := artifacts.closeForRepair(); err != nil {
			result.SecondaryFailures = append(result.SecondaryFailures, SecondaryFailure{Phase: "artifact_close", Message: err.Error(), Cause: err})
			finalizationErr = errors.Join(finalizationErr, err)
		}
		repairFinalizationArtifacts(artifacts, request, result)
		return finalizationErr
	}
	if _, err := artifacts.Finalize(planDigest, baselineTree, finalTree); err != nil {
		result.SecondaryFailures = append(result.SecondaryFailures, SecondaryFailure{Phase: "artifact_manifest", Message: err.Error(), Cause: err})
		result.EvaluationStatus = EvaluationEvaluatorError
		repairFinalizationArtifacts(artifacts, request, result)
		return errors.Join(finalizationErr, err)
	}
	return finalizationErr
}

// hasArtifactFailure identifies retained-evidence failures that make manifest authentication unsafe.
func hasArtifactFailure(failures []SecondaryFailure) bool {
	for _, failure := range failures {
		if strings.HasPrefix(failure.Phase, "artifact_") {
			return true
		}
	}
	return false
}

// adapterCommandCount uses the adapter's complete observed count when bounded retention omitted result events.
func adapterCommandCount(turnEvents []Event, result AgentResult) uint64 {
	count := uint64(countCommandEvents(turnEvents))
	if result.Telemetry != nil {
		return count + result.Telemetry.CommandsObserved
	}
	return count + uint64(countCommandEvents(result.Events))
}

// cloneProviderTelemetry keeps result reports independent from adapter-owned mutable storage.
func cloneProviderTelemetry(telemetry *ProviderTelemetry) *ProviderTelemetry {
	if telemetry == nil {
		return nil
	}
	cloned := *telemetry
	return &cloned
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

// countCommandEvents counts command starts without double-counting completion events.
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
func pathsOverlap(left, right string) (bool, error) {
	if strings.TrimSpace(left) == "" || strings.TrimSpace(right) == "" {
		return false, nil
	}
	left, err := canonicalPathForOverlap(left)
	if err != nil {
		return false, err
	}
	right, err = canonicalPathForOverlap(right)
	if err != nil {
		return false, err
	}
	within := func(parent, child string) bool {
		relative, err := filepath.Rel(parent, child)
		return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
	}
	return within(left, right) || within(right, left), nil
}

// canonicalPathForOverlap resolves every existing ancestor while retaining a missing leaf path.
func canonicalPathForOverlap(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	abs = filepath.Clean(abs)
	missing := []string{}
	for current := abs; ; current = filepath.Dir(current) {
		_, err := os.Lstat(current)
		if err == nil {
			resolved, err := filepath.EvalSymlinks(current)
			if err != nil {
				return "", fmt.Errorf("resolve existing ancestor %q: %w", current, err)
			}
			return filepath.Join(append([]string{resolved}, missing...)...), nil
		}
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("inspect path ancestor %q: %w", current, err)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("resolve path ancestor %q: %w", current, err)
		}
		missing = append([]string{filepath.Base(current)}, missing...)
	}
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
			result.SecondaryFailures = append(result.SecondaryFailures, SecondaryFailure{Phase: "event_capture", Message: err.Error(), Cause: err})
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
func effectiveBackendCapabilities(backendCapabilities, agentProperties []Capability) []Capability {
	available := intersectCapabilities(backendCapabilities)
	if capabilityAvailable(agentProperties, CapabilityFinalResponseCapture) {
		available = append(available, CapabilityFinalResponseCapture)
	}
	if capabilityAvailable(agentProperties, CapabilityCredentialIsolation) {
		return sortedUniqueCapabilities(available)
	}
	filtered := available[:0]
	for _, capability := range available {
		if capability != CapabilityCredentialIsolation {
			filtered = append(filtered, capability)
		}
	}
	return sortedUniqueCapabilities(filtered)
}

// sortedUniqueCapabilities keeps composite backend and adapter properties stable for preflight and reports.
func sortedUniqueCapabilities(capabilities []Capability) []Capability {
	seen := make(map[Capability]bool, len(capabilities))
	result := make([]Capability, 0, len(capabilities))
	for _, capability := range capabilities {
		if seen[capability] {
			continue
		}
		seen[capability] = true
		result = append(result, capability)
	}
	sort.Slice(result, func(left, right int) bool { return result[left] < result[right] })
	return result
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
		case EventFileRead, EventFileWrite:
			observedPath := strings.TrimSpace(strings.ReplaceAll(event.Fields[EventFieldPath], "\\", "/"))
			cleanedPath := path.Clean(observedPath)
			firstSegment := strings.SplitN(observedPath, "/", 2)[0]
			if observedPath == "" || cleanedPath == "." || path.IsAbs(observedPath) || strings.Contains(firstSegment, ":") || strings.HasPrefix(cleanedPath, "../") {
				return fmt.Errorf("supervisor file event %d has an invalid Project-relative path", event.Sequence)
			}
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
		return []SecondaryFailure{{Phase: phase, Message: err.Error(), Cause: err}}
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
