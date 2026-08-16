package eval

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// hasSecondaryFailurePhase reports whether one lifecycle phase reached retained failure evidence.
func hasSecondaryFailurePhase(failures []SecondaryFailure, phase string) bool {
	for _, failure := range failures {
		if failure.Phase == phase {
			return true
		}
	}
	return false
}

// fakePreparer records whether preflight reached Project mutation.
type fakePreparer struct {
	capabilities  PreparationCapabilities
	plan          ResolvedPreparationPlan
	planDigests   []string
	project       *fakePreparedProject
	resolveCalls  int
	prepareCalls  int
	guidanceCalls int
}

// fakeGuidanceResolver returns one attributable treatment after Project preparation.
type fakeGuidanceResolver struct {
	profile string
	calls   int
}

// Resolve records profile resolution against the prepared Project.
func (resolver *fakeGuidanceResolver) Resolve(_ context.Context, profile string, result PreparationResult) (Guidance, error) {
	resolver.calls++
	if result.ProjectRoot == "" {
		return Guidance{}, errors.New("prepared Project is required")
	}
	return Guidance{Profile: profile, Files: map[string][]byte{}}, nil
}

// Capabilities returns deterministic schema and preparation capabilities.
func (preparer *fakePreparer) Capabilities(context.Context) (PreparationCapabilities, error) {
	return preparer.capabilities, nil
}

// Resolve returns the trusted fixture plan without mutating a Project.
func (preparer *fakePreparer) Resolve(_ context.Context, request PreparationRequest) (ResolvedPreparationPlan, error) {
	preparer.resolveCalls++
	plan := preparer.plan
	if preparer.resolveCalls <= len(preparer.planDigests) {
		plan.PlanDigest = preparer.planDigests[preparer.resolveCalls-1]
	}
	plan.ResolutionID = request.OrchestrationID
	return plan, nil
}

// Prepare returns the selected fake Project and records the mutation boundary.
func (preparer *fakePreparer) Prepare(_ context.Context, _ PreparationRequest, plan ResolvedPreparationPlan) (PreparedProject, error) {
	preparer.prepareCalls++
	preparer.project.result.ResolutionID = plan.ResolutionID
	preparer.project.result.PlanDigest = plan.PlanDigest
	return preparer.project, nil
}

// MaterializeGuidance records the Project-owned treatment boundary used by runner tests.
func (preparer *fakePreparer) MaterializeGuidance(_ context.Context, project PreparedProject, guidance Guidance) (Guidance, error) {
	preparer.guidanceCalls++
	if project == nil || project.Result().ProjectRoot == "" {
		return Guidance{}, errors.New("prepared Project is required")
	}
	return guidance, nil
}

// fakePreparedProject owns one deterministic preparation result.
type fakePreparedProject struct {
	result   PreparationResult
	closeLog *[]string
	closeErr error
}

// Result returns exact fixture provenance.
func (project *fakePreparedProject) Result() PreparationResult {
	return project.result
}

// Close records fresh-context cleanup in lifecycle order.
func (project *fakePreparedProject) Close(ctx context.Context) error {
	if ctx.Err() != nil {
		return errors.New("prepared Project cleanup received a cancelled context")
	}
	*project.closeLog = append(*project.closeLog, "project")
	return project.closeErr
}

// fakeBackend opens one deterministic environment around the prepared Project.
type fakeBackend struct {
	capabilities []Capability
	environment  *fakeBackendEnvironment
	openCalls    int
	lastRequest  BackendRequest
}

// Name returns the diagnostic backend identity.
func (backend *fakeBackend) Name() string {
	return "fake"
}

// Capabilities returns trusted observation classes.
func (backend *fakeBackend) Capabilities(context.Context) ([]Capability, error) {
	return append([]Capability(nil), backend.capabilities...), nil
}

// Open records acquisition of the execution boundary.
func (backend *fakeBackend) Open(_ context.Context, request BackendRequest) (BackendEnvironment, error) {
	backend.openCalls++
	backend.lastRequest = request
	return backend.environment, nil
}

// fakeBackendEnvironment owns one private agent namespace.
type fakeBackendEnvironment struct {
	environment RunEnvironment
	baseline    BaselineSnapshot
	events      []Event
	sealed      SealedProject
	closeLog    *[]string
	closeErr    error
}

// Seal records the candidate snapshot only after the agent session has closed.
func (environment *fakeBackendEnvironment) Seal(context.Context) (SealedProject, error) {
	if len(*environment.closeLog) == 0 || (*environment.closeLog)[0] != "session" {
		return SealedProject{}, errors.New("candidate sealed before session cleanup")
	}
	return environment.sealed, nil
}

// Environment returns the adapter-visible paths and capabilities.
func (environment *fakeBackendEnvironment) Environment() RunEnvironment {
	return environment.environment
}

// Baseline returns the supervisor-owned treatment baseline configured by the test.
func (environment *fakeBackendEnvironment) Baseline(context.Context) (BaselineSnapshot, error) {
	return environment.baseline, nil
}

// ObservedEvents returns backend-owned observations rather than adapter telemetry.
func (environment *fakeBackendEnvironment) ObservedEvents(context.Context) ([]Event, error) {
	return append([]Event(nil), environment.events...), nil
}

// Close records fresh-context backend destruction.
func (environment *fakeBackendEnvironment) Close(ctx context.Context) error {
	if ctx.Err() != nil {
		return errors.New("backend cleanup received a cancelled context")
	}
	*environment.closeLog = append(*environment.closeLog, "backend")
	return environment.closeErr
}

// fakeAgent provides one prepared identity and session.
type fakeAgent struct {
	capabilities      []Capability
	preparation       *fakeAgentPreparation
	session           *fakeSession
	prepareHook       func(RunEnvironment, Guidance) error
	prepareCalls      int
	startCalls        int
	sessionIdentities []AgentSessionIdentity
	startErr          error
}

// Name returns the fake adapter identity.
func (agent *fakeAgent) Name() string {
	return "fake-agent"
}

// Properties returns the adapter's enforced safety properties.
func (agent *fakeAgent) Properties(context.Context) (AgentProperties, error) {
	return AgentProperties{Properties: append([]Capability(nil), agent.capabilities...)}, nil
}

// Prepare records private-home and guidance preparation.
func (agent *fakeAgent) Prepare(_ context.Context, environment RunEnvironment, guidance Guidance) (AgentPreparation, error) {
	agent.prepareCalls++
	if agent.prepareHook != nil {
		if err := agent.prepareHook(environment, guidance); err != nil {
			return nil, err
		}
	}
	return agent.preparation, nil
}

// Start returns one fresh fake provider session.
func (agent *fakeAgent) Start(context.Context, PreparedAgent) (EvaluationSession, error) {
	if agent.startCalls < len(agent.sessionIdentities) {
		agent.session.identity = agent.sessionIdentities[agent.startCalls]
	}
	agent.startCalls++
	return agent.session, agent.startErr
}

// fakeAgentPreparation owns the private adapter home.
type fakeAgentPreparation struct {
	agent    PreparedAgent
	closeLog *[]string
	closeErr error
}

// Agent returns the attributable executable and model identity.
func (preparation *fakeAgentPreparation) Agent() PreparedAgent {
	return preparation.agent
}

// Close records fresh-context private-home cleanup.
func (preparation *fakeAgentPreparation) Close(ctx context.Context) error {
	if ctx.Err() != nil {
		return errors.New("agent preparation cleanup received a cancelled context")
	}
	*preparation.closeLog = append(*preparation.closeLog, "agent_preparation")
	return preparation.closeErr
}

// fakeSession emits selected events and terminal behavior.
type fakeSession struct {
	turn           AgentTurnResult
	turnErr        error
	result         AgentResult
	waitErr        error
	waitForContext bool
	closeLog       *[]string
	closeErr       error
	lastTurn       AgentTurn
	identity       AgentSessionIdentity
}

// Identity returns the effective fake provider identity established at session start.
func (session *fakeSession) Identity() AgentSessionIdentity {
	if session.identity != (AgentSessionIdentity{}) {
		return session.identity
	}
	return AgentSessionIdentity{Version: "fake-agent/1", Model: "fake-model", ModelProvider: "fake-provider"}
}

// capturingVerifier records the exact sealed input passed across the verifier boundary.
type capturingVerifier struct {
	input  VerificationInput
	result VerificationResult
}

// ID returns the manifest's promoted verifier identity.
func (*capturingVerifier) ID() string {
	return "add-http-controller/v1"
}

// Capabilities requires no additional observation evidence.
func (*capturingVerifier) Capabilities() []Capability {
	return nil
}

// Verify records the sealed tree and returns a passing framework outcome.
func (verifier *capturingVerifier) Verify(_ context.Context, input VerificationInput) (VerificationResult, error) {
	verifier.input = input
	if verifier.result.FrameworkOutcome.ID != "" {
		return verifier.result, nil
	}
	return VerificationResult{FrameworkOutcome: EndpointResult{ID: "framework", Status: EndpointPassed}}, nil
}

// Turn records prompt delivery through its configured result.
func (session *fakeSession) Turn(_ context.Context, turn AgentTurn) (AgentTurnResult, error) {
	session.lastTurn = turn
	return session.turn, session.turnErr
}

// Wait returns the configured provider completion.
func (session *fakeSession) Wait(ctx context.Context) (AgentResult, error) {
	if session.waitForContext {
		<-ctx.Done()
		return AgentResult{}, ctx.Err()
	}
	return session.result, session.waitErr
}

// Close records fresh-context descendant-job cleanup.
func (session *fakeSession) Close(ctx context.Context) error {
	if ctx.Err() != nil {
		return errors.New("session cleanup received a cancelled context")
	}
	*session.closeLog = append(*session.closeLog, "session")
	return session.closeErr
}

// TestRunnerCompletesLifecycleAndCleansInReverseOrder verifies the deterministic happy path.
func TestRunnerCompletesLifecycleAndCleansInReverseOrder(t *testing.T) {
	runner, closeLog, preparer, backend, agent := newFakeRunner(t)
	result, err := runner.Run(context.Background(), fakeAttemptRequest())
	if err != nil {
		t.Fatalf("Run(): %v", err)
	}
	if result.AgentOutcome != AgentCompleted || result.EvaluationStatus != EvaluationValid {
		t.Fatalf("attempt result = %#v", result)
	}
	if result.EvaluationID == "" || result.PromptDigest == "" || result.GuidanceDigest == "" || result.ScenarioSchema != 2 || result.PlanDigest == "" || result.ScenarioPlanDigest == "" || result.CatalogDigest == "" || len(result.DependencyDigests) == 0 || result.ProjectConfigDigest == "" || result.EnvironmentDigest == "" || result.PreparedTree == "" || result.BaselineTree == "" || result.FinalTree == "" {
		t.Fatalf("attempt provenance is incomplete: %#v", result)
	}
	if result.GuidanceProfile != GuidanceProfileNone || result.ForjDigest != "sha256:forj" || result.Backend != "fake" || result.Agent != "fake-agent" || result.AgentDigest != "sha256:agent" || result.AgentVersion != "fake-agent/1" || result.Model != "fake-model" || result.ModelProvider != "fake-provider" {
		t.Fatalf("attempt identity = %#v", result)
	}
	if !reflect.DeepEqual(result.Runtime, fakeAttemptRequest().Runtime) {
		t.Fatalf("runtime identity = %#v", result.Runtime)
	}
	for _, name := range []string{"commands.jsonl", "diff.patch", "environment.json", "events.jsonl", "manifest.json", "run.json", "scorecard.json", "summary.txt", "transcript.redacted.txt", "verification.json"} {
		if _, err := os.Stat(filepath.Join(runner.Artifacts.root, result.AttemptID, name)); err != nil {
			t.Fatalf("retained artifact %s: %v", name, err)
		}
	}
	summary, err := os.ReadFile(filepath.Join(runner.Artifacts.root, result.AttemptID, "summary.txt"))
	if err != nil || strings.Contains(string(summary), "Diagnostic limitation:") || !strings.Contains(string(summary), "Result verified") {
		t.Fatalf("summary = %q, %v", summary, err)
	}
	environment, err := os.ReadFile(filepath.Join(runner.Artifacts.root, result.AttemptID, "environment.json"))
	if err != nil || !strings.Contains(string(environment), `"requested_shell_network": "off"`) || strings.Contains(string(environment), `"shell_network":`) {
		t.Fatalf("environment = %q, %v", environment, err)
	}
	wantMilestones := []Milestone{MilestonePreflight, MilestoneProviderSessionStarted, MilestonePromptDelivered, MilestoneFirstAgentAction, MilestoneAgentTerminal, MilestoneEvaluationTerminal}
	if !reflect.DeepEqual(result.Milestones, wantMilestones) {
		t.Fatalf("milestones = %q, want %q", result.Milestones, wantMilestones)
	}
	if !reflect.DeepEqual(*closeLog, []string{"session", "agent_preparation", "backend", "project"}) {
		t.Fatalf("cleanup order = %q", *closeLog)
	}
	if preparer.resolveCalls != 1 || preparer.prepareCalls != 1 || backend.openCalls != 1 || agent.prepareCalls != 1 {
		t.Fatalf("lifecycle calls = resolve:%d prepare:%d open:%d agent:%d", preparer.resolveCalls, preparer.prepareCalls, backend.openCalls, agent.prepareCalls)
	}
	if backend.lastRequest.Project != preparer.project || backend.lastRequest.ShellNetwork != "off" || backend.lastRequest.CommandLimit != fakeAttemptRequest().Definition.Limits.Commands {
		t.Fatalf("backend request = %#v", backend.lastRequest)
	}
	if agent.session.lastTurn.Prompt != fakeAttemptRequest().Definition.Prompt || !reflect.DeepEqual(agent.session.lastTurn.Limits, fakeAttemptRequest().Definition.Limits) {
		t.Fatalf("agent turn = %#v", agent.session.lastTurn)
	}
}

// TestRunnerPromotesAcceptedClarificationToAbstention keeps provider completion separate until the verifier accepts the exact response.
func TestRunnerPromotesAcceptedClarificationToAbstention(t *testing.T) {
	runner, _, _, _, agent := newFakeRunner(t)
	verifier := runner.Registry.verifiers["add-http-controller/v1"].(*capturingVerifier)
	verifier.result = VerificationResult{
		FrameworkOutcome: EndpointResult{ID: "safe-abstention", Status: EndpointPassed},
		Abstention:       &EndpointResult{ID: "clarification", Status: EndpointPassed},
	}
	agent.session.result.Message = "captured terminal response"
	result, err := runner.Run(context.Background(), fakeAttemptRequest())
	if err != nil {
		t.Fatalf("Run(): %v", err)
	}
	if verifier.input.FinalResponse != "captured terminal response" {
		t.Fatalf("FinalResponse = %q", verifier.input.FinalResponse)
	}
	if result.AgentOutcome != AgentAbstained || result.EvaluationStatus != EvaluationValidAbstention {
		t.Fatalf("result = %#v", result)
	}
}

// TestEffectiveBackendCapabilitiesIncludesExactFinalResponseCapture keeps adapter claims from promoting action telemetry.
func TestEffectiveBackendCapabilitiesIncludesExactFinalResponseCapture(t *testing.T) {
	capabilities := effectiveBackendCapabilities(
		[]Capability{CapabilityCommands, CapabilityFileReads, CapabilityCredentialIsolation},
		[]Capability{CapabilityFinalResponseCapture},
	)
	want := []Capability{CapabilityCommands, CapabilityFileReads, CapabilityFinalResponseCapture}
	if !reflect.DeepEqual(capabilities, want) {
		t.Fatalf("capabilities = %v, want %v", capabilities, want)
	}
}

// TestRunnerRepairsTerminalReportsAfterFinalizationFailure keeps durable reports aligned with the returned integrity failure.
func TestRunnerRepairsTerminalReportsAfterFinalizationFailure(t *testing.T) {
	runner, _, _, _, agent := newFakeRunner(t)
	request := fakeAttemptRequest()
	agent.prepareHook = func(RunEnvironment, Guidance) error {
		return os.Mkdir(filepath.Join(runner.Artifacts.root, request.AttemptID, "unexpected"), 0o700)
	}
	result, err := runner.Run(context.Background(), request)
	if err == nil || result.EvaluationStatus != EvaluationEvaluatorError {
		t.Fatalf("Run() = %#v, %v, want finalization failure", result, err)
	}
	if !hasSecondaryFailurePhase(result.SecondaryFailures, "artifact_manifest") {
		t.Fatalf("secondary failures = %#v, want artifact_manifest", result.SecondaryFailures)
	}
	directory := filepath.Join(runner.Artifacts.root, request.AttemptID)
	var retained AttemptResult
	body, readErr := os.ReadFile(filepath.Join(directory, "run.json"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if err := json.Unmarshal(body, &retained); err != nil {
		t.Fatal(err)
	}
	if retained.EvaluationStatus != EvaluationEvaluatorError || !hasSecondaryFailurePhase(retained.SecondaryFailures, "artifact_manifest") {
		t.Fatalf("retained run = %#v, want finalization failure", retained)
	}
	var scorecard AttemptScorecard
	body, readErr = os.ReadFile(filepath.Join(directory, "scorecard.json"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if err := json.Unmarshal(body, &scorecard); err != nil {
		t.Fatal(err)
	}
	if scorecard.EvaluationStatus != EvaluationEvaluatorError {
		t.Fatalf("retained scorecard = %#v, want evaluator_error", scorecard)
	}
	summary, readErr := os.ReadFile(filepath.Join(directory, "summary.txt"))
	if readErr != nil || !strings.Contains(string(summary), "Evaluation failed") {
		t.Fatalf("retained summary = %q, %v", summary, readErr)
	}
	if _, statErr := os.Stat(filepath.Join(directory, "triage.json")); statErr != nil {
		t.Fatalf("retained triage: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(directory, "manifest.json")); !os.IsNotExist(statErr) {
		t.Fatalf("manifest after failed finalization = %v, want absent", statErr)
	}
}

// TestRunnerCapturesBaselineAfterGuidanceMaterialization keeps treatment files out of the agent-authored diff.
func TestRunnerCapturesBaselineAfterGuidanceMaterialization(t *testing.T) {
	runner, _, preparer, backend, agent := newFakeRunner(t)
	backend.environment.sealed.Root = preparer.project.result.ProjectRoot
	backend.environment.baseline.TreeDigest = ""
	agent.prepareHook = func(environment RunEnvironment, _ Guidance) error {
		if err := os.WriteFile(filepath.Join(environment.ProjectRoot, "AGENTS.md"), []byte("Use GoForj generators.\n"), 0o644); err != nil {
			return err
		}
		_, digest, err := snapshotProjectForDiff(environment.ProjectRoot)
		backend.environment.baseline.TreeDigest = digest
		return err
	}
	request := fakeAttemptRequest()
	request.GuidanceProfile = GuidanceProfileAgents
	result, err := runner.Run(context.Background(), request)
	if err != nil {
		t.Fatalf("Run(): %v", err)
	}
	_, wantBaseline, err := snapshotProjectForDiff(preparer.project.result.ProjectRoot)
	if err != nil {
		t.Fatalf("snapshotProjectForDiff(): %v", err)
	}
	if result.PreparedTree != "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" || result.BaselineTree != wantBaseline {
		t.Fatalf("tree identities = prepared %q baseline %q, want prepared source and materialized treatment %q", result.PreparedTree, result.BaselineTree, wantBaseline)
	}
	diff, err := os.ReadFile(filepath.Join(runner.Artifacts.root, result.AttemptID, "diff.patch"))
	if err != nil {
		t.Fatalf("read retained diff: %v", err)
	}
	if strings.Contains(string(diff), "AGENTS.md") {
		t.Fatalf("treatment guidance appeared as agent-authored change:\n%s", diff)
	}
}

// TestRunnerRejectsPreparedTreeDigestDrift keeps the supervisor and GoForj preparation boundary on one tree identity contract.
func TestRunnerRejectsPreparedTreeDigestDrift(t *testing.T) {
	runner, _, preparer, _, _ := newFakeRunner(t)
	preparer.project.result.BaselineTree = "sha256:incompatible"

	result, err := runner.Run(context.Background(), fakeAttemptRequest())
	if err == nil || !strings.Contains(err.Error(), "incompatible digest") {
		t.Fatalf("Run() error = %v, want prepared tree digest rejection", err)
	}
	if result.EvaluationStatus != EvaluationFixtureError || result.AgentOutcome != AgentNotStarted {
		t.Fatalf("attempt result = %#v", result)
	}
}

// TestDigestGuidanceIsStableAcrossMapAndSelectionOrder keeps treatment identity independent from Go iteration order.
func TestDigestGuidanceIsStableAcrossMapAndSelectionOrder(t *testing.T) {
	left := Guidance{
		Profile: GuidanceProfileAgents,
		Files:   map[string][]byte{"AGENTS.md": []byte("root"), "nested/AGENTS.md": []byte("nested")},
		Skills:  []string{"second", "first"},
		MCP:     []string{"docs", "project"},
	}
	right := Guidance{
		Profile: GuidanceProfileAgents,
		Files:   map[string][]byte{"nested/AGENTS.md": []byte("nested"), "AGENTS.md": []byte("root")},
		Skills:  []string{"first", "second"},
		MCP:     []string{"project", "docs"},
	}
	if digestGuidance(left) != digestGuidance(right) {
		t.Fatal("equivalent guidance produced different digests")
	}
	right.Files["AGENTS.md"] = []byte("changed")
	if digestGuidance(left) == digestGuidance(right) {
		t.Fatal("changed guidance retained the same digest")
	}
}

// TestRunnerClosesAgentBeforeVerifyingSealedTree prevents live provider processes from racing verifier inspection.
func TestRunnerClosesAgentBeforeVerifyingSealedTree(t *testing.T) {
	runner, _, _, backend, _ := newFakeRunner(t)
	verifier := &capturingVerifier{}
	registry, err := NewRegistry(PromotedWorkflows(), []Verifier{verifier})
	if err != nil {
		t.Fatalf("NewRegistry(): %v", err)
	}
	runner.Registry = registry
	result, err := runner.Run(context.Background(), fakeAttemptRequest())
	if err != nil {
		t.Fatalf("Run(): %v", err)
	}
	if verifier.input.ProjectRoot != backend.environment.sealed.Root || verifier.input.FinalTree != "sha256:final-tree" || verifier.input.BaselineTree != result.BaselineTree {
		t.Fatalf("verification input = %#v", verifier.input)
	}
	if len(verifier.input.Changes) != 0 {
		t.Fatalf("unchanged Project changes = %#v", verifier.input.Changes)
	}
}

// TestRunnerPassesSupervisorComputedChanges gives verifiers structured ownership evidence instead of a display patch.
func TestRunnerPassesSupervisorComputedChanges(t *testing.T) {
	runner, _, _, backend, _ := newFakeRunner(t)
	if err := os.WriteFile(filepath.Join(backend.environment.sealed.Root, "wire_gen.go"), []byte("generated"), 0o644); err != nil {
		t.Fatal(err)
	}
	verifier := &capturingVerifier{}
	registry, err := NewRegistry(PromotedWorkflows(), []Verifier{verifier})
	if err != nil {
		t.Fatal(err)
	}
	runner.Registry = registry
	if _, err := runner.Run(context.Background(), fakeAttemptRequest()); err != nil {
		t.Fatalf("Run(): %v", err)
	}
	if len(verifier.input.Changes) != 1 || verifier.input.Changes[0].Path != "wire_gen.go" || verifier.input.Changes[0].Before.Kind != "" || verifier.input.Changes[0].After.Kind != "file" || verifier.input.Changes[0].After.Digest == "" {
		t.Fatalf("changes = %#v", verifier.input.Changes)
	}
}

// TestRunnerClassifiesTypedStartAndTurnFailures keeps provider faults distinct from adapter faults.
func TestRunnerClassifiesTypedStartAndTurnFailures(t *testing.T) {
	for _, test := range []struct {
		name  string
		setup func(*fakeAgent)
		want  AgentOutcome
	}{
		{name: "start provider", setup: func(agent *fakeAgent) {
			agent.startErr = &AgentFailure{Outcome: AgentProviderError, Err: errors.New("provider unavailable")}
		}, want: AgentProviderError},
		{name: "turn provider", setup: func(agent *fakeAgent) {
			agent.session.turnErr = &AgentFailure{Outcome: AgentProviderError, Err: errors.New("provider rejected prompt")}
		}, want: AgentProviderError},
	} {
		t.Run(test.name, func(t *testing.T) {
			runner, _, _, _, agent := newFakeRunner(t)
			test.setup(agent)
			result, err := runner.Run(context.Background(), fakeAttemptRequest())
			if err == nil || result.AgentOutcome != test.want {
				t.Fatalf("Run() = %#v, %v", result, err)
			}
		})
	}
}

// TestRunnerRejectsCommandBudgetOverrun verifies provider adapters cannot silently exceed manifest policy.
func TestRunnerRejectsCommandBudgetOverrun(t *testing.T) {
	runner, _, _, backend, _ := newFakeRunner(t)
	request := fakeAttemptRequest()
	request.Definition.Limits.Commands = 1
	backend.environment.events = []Event{
		{Sequence: 1, Source: EventSourceSupervisor, Kind: EventCommandStarted, Fields: map[string]string{EventFieldCommandID: "command-1", EventFieldExecutableDigest: "sha256:forj", EventFieldArguments: `[]`}},
		{Sequence: 2, Source: EventSourceSupervisor, Kind: EventCommandFinished, Fields: map[string]string{EventFieldCommandID: "command-1", EventFieldExitCode: "0"}},
		{Sequence: 3, Source: EventSourceSupervisor, Kind: EventCommandStarted, Fields: map[string]string{EventFieldCommandID: "command-2", EventFieldExecutableDigest: "sha256:forj", EventFieldArguments: `[]`}},
		{Sequence: 4, Source: EventSourceSupervisor, Kind: EventCommandFinished, Fields: map[string]string{EventFieldCommandID: "command-2", EventFieldExitCode: "0"}},
		{Sequence: 5, Source: EventSourceSupervisor, Kind: EventRunFinished},
	}

	result, err := runner.Run(context.Background(), request)
	if err == nil || !strings.Contains(err.Error(), "exceeded command limit 1") {
		t.Fatalf("Run() = %#v, %v, want command budget failure", result, err)
	}
	if result.AgentOutcome != AgentAdapterError || result.EvaluationStatus != EvaluationEvaluatorError {
		t.Fatalf("attempt result = %#v", result)
	}
}

// TestRunnerRejectsMissingCapabilityBeforeMutation keeps diagnostic gaps out of behavioral denominators.
func TestRunnerRejectsMissingCapabilityBeforeMutation(t *testing.T) {
	runner, closeLog, preparer, backend, agent := newFakeRunner(t)
	backend.capabilities = nil
	result, err := runner.Run(context.Background(), fakeAttemptRequest())
	if err != nil {
		t.Fatalf("Run(): %v", err)
	}
	if result.EvaluationStatus != EvaluationIneligible || len(result.UnavailableEvidence) != len(authoritativeCapabilities)+1 || !capabilityAvailable(result.UnavailableEvidence, CapabilityCommands) {
		t.Fatalf("ineligible result = %#v", result)
	}
	if preparer.resolveCalls != 1 || preparer.prepareCalls != 0 || backend.openCalls != 0 || agent.prepareCalls != 0 || len(*closeLog) != 0 {
		t.Fatalf("preflight mutated resources: resolve:%d prepare:%d open:%d agent:%d cleanup:%q", preparer.resolveCalls, preparer.prepareCalls, backend.openCalls, agent.prepareCalls, *closeLog)
	}
}

// TestRunnerRejectsUnknownIntent prevents a caller typo from weakening authoritative capability gates.
func TestRunnerRejectsUnknownIntent(t *testing.T) {
	runner, closeLog, preparer, backend, agent := newFakeRunner(t)
	request := fakeAttemptRequest()
	request.Intent = "typo"
	result, err := runner.Run(context.Background(), request)
	if err == nil || !strings.Contains(err.Error(), "intent") || result.EvaluationStatus != EvaluationEvaluatorError {
		t.Fatalf("Run() = %#v, %v", result, err)
	}
	if preparer.resolveCalls != 0 || preparer.prepareCalls != 0 || backend.openCalls != 0 || agent.prepareCalls != 0 || len(*closeLog) != 0 {
		t.Fatalf("invalid intent mutated resources: resolve:%d prepare:%d open:%d agent:%d cleanup:%q", preparer.resolveCalls, preparer.prepareCalls, backend.openCalls, agent.prepareCalls, *closeLog)
	}
}

// TestRunnerDiagnosticRejectsAdapterCommandOverflow keeps diagnostic telemetry bounded without treating it as trusted evidence.
func TestRunnerDiagnosticRejectsAdapterCommandOverflow(t *testing.T) {
	runner, _, preparer, backend, agent := newFakeRunner(t)
	agent.capabilities = nil
	backend.capabilities = nil
	request := fakeAttemptRequest()
	request.Intent = IntentDiagnostic
	request.Definition.Limits.Commands = 1
	agent.session.turn.Events = []Event{
		{Sequence: 1, Kind: EventCommandStarted, Fields: map[string]string{EventFieldCommandID: "adapter-command-1"}},
		{Sequence: 2, Kind: EventCommandStarted, Fields: map[string]string{EventFieldCommandID: "adapter-command-2"}},
	}

	result, err := runner.Run(context.Background(), request)
	if err == nil || !strings.Contains(err.Error(), "adapter telemetry exceeded command limit 1") {
		t.Fatalf("Run() = %#v, %v, want diagnostic command budget failure", result, err)
	}
	if result.EvaluationStatus != EvaluationDiagnostic || result.AgentOutcome != AgentAdapterError || result.Verification != nil {
		t.Fatalf("diagnostic result = %#v", result)
	}
	if len(result.UnavailableEvidence) != len(authoritativeCapabilities)+1 || !capabilityAvailable(result.UnavailableEvidence, CapabilityCommands) {
		t.Fatalf("unavailable evidence = %q", result.UnavailableEvidence)
	}
	if preparer.prepareCalls != 1 || backend.openCalls != 1 || agent.prepareCalls != 1 {
		t.Fatalf("diagnostic lifecycle calls = prepare:%d open:%d agent:%d", preparer.prepareCalls, backend.openCalls, agent.prepareCalls)
	}
	commands, err := os.ReadFile(filepath.Join(runner.Artifacts.root, request.AttemptID, "commands.jsonl"))
	if err != nil {
		t.Fatalf("read diagnostic commands: %v", err)
	}
	if !bytes.Contains(commands, []byte(`"source":"adapter"`)) {
		t.Fatalf("diagnostic commands = %s, want adapter telemetry", commands)
	}
}

// TestRunnerDiagnosticUsesCompleteCommandTelemetryAfterTruncation avoids undercounting commands omitted by bounded retention.
func TestRunnerDiagnosticUsesCompleteCommandTelemetryAfterTruncation(t *testing.T) {
	runner, _, _, backend, agent := newFakeRunner(t)
	agent.capabilities = nil
	backend.capabilities = nil
	request := fakeAttemptRequest()
	request.Intent = IntentDiagnostic
	request.Definition.Limits.Commands = 1
	agent.session.turn.Events = nil
	agent.session.result.Events = []Event{{Sequence: 1, Kind: EventCommandStarted, Fields: map[string]string{EventFieldCommandID: "retained-command"}}}
	agent.session.result.Telemetry = &ProviderTelemetry{EventsObserved: 2, EventsDropped: 1, CommandsObserved: 2}
	agent.session.waitErr = errors.New("provider telemetry truncated")

	result, err := runner.Run(context.Background(), request)
	if err == nil || !strings.Contains(err.Error(), "adapter telemetry exceeded command limit 1") {
		t.Fatalf("Run() = %#v, %v, want complete command budget failure", result, err)
	}
	if result.ProviderTelemetry == nil || result.ProviderTelemetry.CommandsObserved != 2 || result.AgentOutcome != AgentAdapterError {
		t.Fatalf("diagnostic telemetry result = %#v", result)
	}
}

// TestRunnerAuthoritativeRequiresIsolationBaseline prevents a partially observed backend from entering an authoritative denominator.
func TestRunnerAuthoritativeRequiresIsolationBaseline(t *testing.T) {
	runner, closeLog, preparer, backend, agent := newFakeRunner(t)
	backend.capabilities = []Capability{CapabilityCommands}
	request := fakeAttemptRequest()
	request.Intent = IntentAuthoritative

	result, err := runner.Run(context.Background(), request)
	if err != nil {
		t.Fatalf("Run(): %v", err)
	}
	if result.EvaluationStatus != EvaluationIneligible || len(result.UnavailableEvidence) != len(authoritativeCapabilities) {
		t.Fatalf("authoritative preflight = %#v", result)
	}
	if preparer.prepareCalls != 0 || backend.openCalls != 0 || agent.prepareCalls != 0 || len(*closeLog) != 0 {
		t.Fatalf("authoritative preflight mutated resources: prepare:%d open:%d agent:%d cleanup:%q", preparer.prepareCalls, backend.openCalls, agent.prepareCalls, *closeLog)
	}
}

// TestRunnerAuthoritativeRequiresAdapterCredentialIsolation prevents a file-backed credential adapter from inheriting a backend's claim.
func TestRunnerAuthoritativeRequiresAdapterCredentialIsolation(t *testing.T) {
	runner, closeLog, preparer, backend, agent := newFakeRunner(t)
	agent.capabilities = []Capability{CapabilityCommands}
	request := fakeAttemptRequest()
	request.Intent = IntentAuthoritative

	result, err := runner.Run(context.Background(), request)
	if err != nil {
		t.Fatalf("Run(): %v", err)
	}
	if result.EvaluationStatus != EvaluationIneligible || !reflect.DeepEqual(result.UnavailableEvidence, []Capability{CapabilityCredentialIsolation}) {
		t.Fatalf("authoritative credential preflight = %#v", result)
	}
	if preparer.prepareCalls != 0 || backend.openCalls != 0 || agent.prepareCalls != 0 || len(*closeLog) != 0 {
		t.Fatalf("credential preflight mutated resources: prepare:%d open:%d agent:%d cleanup:%q", preparer.prepareCalls, backend.openCalls, agent.prepareCalls, *closeLog)
	}
}

// TestRunnerAuthoritativeRequiresCompleteSupervisorBaseline prevents an incomplete snapshot from entering a valid denominator.
func TestRunnerAuthoritativeRequiresCompleteSupervisorBaseline(t *testing.T) {
	runner, _, _, backend, agent := newFakeRunner(t)
	backend.capabilities = append(backend.capabilities, authoritativeCapabilities...)
	agent.capabilities = append(agent.capabilities, CapabilityCredentialIsolation)
	backend.environment.baseline = BaselineSnapshot{}
	request := fakeAttemptRequest()
	request.Intent = IntentAuthoritative
	result, err := runner.Run(context.Background(), request)
	if err != nil || result.EvaluationStatus != EvaluationIneligible || agent.session.lastTurn.Prompt != "" {
		t.Fatalf("Run() = %#v, %v", result, err)
	}
}

// TestRunnerRejectsAdapterProvenanceInSupervisorEvidence ensures an adapter cannot self-attest a behavioral event.
func TestRunnerRejectsAdapterProvenanceInSupervisorEvidence(t *testing.T) {
	runner, _, _, backend, _ := newFakeRunner(t)
	backend.environment.events[0].Source = EventSourceAdapter
	result, err := runner.Run(context.Background(), fakeAttemptRequest())
	if err == nil || result.EvaluationStatus != EvaluationEvaluatorError || !strings.Contains(err.Error(), "supervisor provenance") {
		t.Fatalf("Run() = %#v, %v", result, err)
	}
}

// TestValidateSupervisorEventsRejectsAmbiguousCommandCorrelation prevents monitor defects from satisfying workflow requirements.
func TestValidateSupervisorEventsRejectsAmbiguousCommandCorrelation(t *testing.T) {
	validStart := Event{Sequence: 1, Source: EventSourceSupervisor, Kind: EventCommandStarted, Fields: map[string]string{EventFieldCommandID: "command-1", EventFieldExecutableDigest: "sha256:forj", EventFieldArguments: `[]`}}
	for _, test := range []struct {
		name   string
		events []Event
	}{
		{name: "duplicate sequence", events: []Event{validStart, {Sequence: 1, Source: EventSourceSupervisor, Kind: EventRunFinished}}},
		{name: "duplicate start", events: []Event{validStart, {Sequence: 2, Source: EventSourceSupervisor, Kind: EventCommandStarted, Fields: validStart.Fields}}},
		{name: "unmatched finish", events: []Event{{Sequence: 1, Source: EventSourceSupervisor, Kind: EventCommandFinished, Fields: map[string]string{EventFieldCommandID: "command-1", EventFieldExitCode: "0"}}}},
		{name: "unfinished command", events: []Event{validStart}},
		{name: "invalid arguments", events: []Event{{Sequence: 1, Source: EventSourceSupervisor, Kind: EventCommandStarted, Fields: map[string]string{EventFieldCommandID: "command-1", EventFieldExecutableDigest: "sha256:forj", EventFieldArguments: `{}`}}}},
		{name: "missing file path", events: []Event{{Sequence: 1, Source: EventSourceSupervisor, Kind: EventFileRead}}},
		{name: "escaping file path", events: []Event{{Sequence: 1, Source: EventSourceSupervisor, Kind: EventFileWrite, Fields: map[string]string{EventFieldPath: "../outside"}}}},
		{name: "absolute file path", events: []Event{{Sequence: 1, Source: EventSourceSupervisor, Kind: EventFileRead, Fields: map[string]string{EventFieldPath: "/tmp/outside"}}}},
		{name: "windows absolute file path", events: []Event{{Sequence: 1, Source: EventSourceSupervisor, Kind: EventFileRead, Fields: map[string]string{EventFieldPath: `C:\outside`}}}},
		{name: "terminal is not final", events: []Event{{Sequence: 1, Source: EventSourceSupervisor, Kind: EventRunFinished}, {Sequence: 2, Source: EventSourceSupervisor, Kind: EventMessage}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := validateSupervisorEvents(test.events, false); err == nil {
				t.Fatalf("validateSupervisorEvents(%#v) succeeded", test.events)
			}
		})
	}
	if err := validateSupervisorEvents(nil, true); err == nil {
		t.Fatal("validateSupervisorEvents accepted a missing required terminal marker")
	}
}

// TestRunnerRetainsTriageForFailedVerifiedEndpoint keeps evidence validity from hiding an application failure.
func TestRunnerRetainsTriageForFailedVerifiedEndpoint(t *testing.T) {
	runner, _, _, _, _ := newFakeRunner(t)
	verifier := &capturingVerifier{result: VerificationResult{
		FrameworkOutcome:    EndpointResult{ID: "framework", Status: EndpointFailed},
		WorkflowConformance: EndpointResult{ID: "workflow", Status: EndpointIneligible},
	}}
	registry, err := NewRegistry(PromotedWorkflows(), []Verifier{verifier})
	if err != nil {
		t.Fatalf("NewRegistry(): %v", err)
	}
	runner.Registry = registry
	result, err := runner.Run(context.Background(), fakeAttemptRequest())
	if err != nil {
		t.Fatalf("Run(): %v", err)
	}
	if result.EvaluationStatus != EvaluationValid || result.Verification == nil || result.Verification.FrameworkOutcome.Status != EndpointFailed {
		t.Fatalf("failed verified result = %#v", result)
	}
	body, err := os.ReadFile(filepath.Join(runner.Artifacts.root, result.AttemptID, "triage.json"))
	if err != nil {
		t.Fatalf("read triage artifact: %v", err)
	}
	if !strings.Contains(string(body), `"state": "unreviewed"`) {
		t.Fatalf("triage artifact = %s", body)
	}
}

// TestAttemptNeedsTriageCoversTerminalResults proves every failed endpoint is triaged without treating expected ineligibility as failure.
func TestAttemptNeedsTriageCoversTerminalResults(t *testing.T) {
	failed := EndpointResult{ID: "failed", Status: EndpointFailed}
	passed := EndpointResult{ID: "passed", Status: EndpointPassed}
	ineligible := EndpointResult{ID: "ineligible", Status: EndpointIneligible}
	tests := []struct {
		name   string
		result AttemptResult
		want   bool
	}{
		{name: "evaluator failure", result: AttemptResult{EvaluationStatus: EvaluationEvaluatorError}, want: true},
		{name: "framework failure", result: AttemptResult{EvaluationStatus: EvaluationValid, Verification: &VerificationResult{FrameworkOutcome: failed}}, want: true},
		{name: "workflow failure", result: AttemptResult{EvaluationStatus: EvaluationValid, Verification: &VerificationResult{FrameworkOutcome: passed, WorkflowConformance: failed}}, want: true},
		{name: "contract failure", result: AttemptResult{EvaluationStatus: EvaluationValid, Verification: &VerificationResult{FrameworkOutcome: passed, WorkflowConformance: passed, Contract: &failed}}, want: true},
		{name: "check failure", result: AttemptResult{EvaluationStatus: EvaluationValid, Verification: &VerificationResult{FrameworkOutcome: passed, WorkflowConformance: passed, Checks: []EndpointResult{failed}}}, want: true},
		{name: "expected ineligibility", result: AttemptResult{EvaluationStatus: EvaluationValid, Verification: &VerificationResult{FrameworkOutcome: passed, WorkflowConformance: ineligible}}, want: false},
		{name: "passing result", result: AttemptResult{EvaluationStatus: EvaluationValid, Verification: &VerificationResult{FrameworkOutcome: passed, WorkflowConformance: passed}}, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := attemptNeedsTriage(test.result); got != test.want {
				t.Fatalf("attemptNeedsTriage() = %v, want %v", got, test.want)
			}
		})
	}
}

// TestNewTriageReviewDefersHumanDispositionOutsideImmutableArtifacts preserves the original authenticated evidence set.
func TestNewTriageReviewDefersHumanDispositionOutsideImmutableArtifacts(t *testing.T) {
	review, err := NewTriageReview("attempt-01", TriageDisposition("guidance_discoverability"), "reviewer", time.Unix(1, 0))
	if err != nil || review.AttemptID != "attempt-01" || review.ReviewedAt.Location() != time.UTC {
		t.Fatalf("NewTriageReview() = %#v, %v", review, err)
	}
	if _, err := NewTriageReview("", TriageDisposition("guidance_discoverability"), "reviewer", time.Time{}); err == nil {
		t.Fatal("NewTriageReview() accepted incomplete review")
	}
}

// TestRunnerRejectsArtifactProjectOverlap prevents the candidate namespace from containing supervisor evidence.
func TestRunnerRejectsArtifactProjectOverlap(t *testing.T) {
	runner, _, preparer, backend, agent := newFakeRunner(t)
	request := fakeAttemptRequest()
	request.Preparation.DestinationRoot = runner.Artifacts.root
	result, err := runner.Run(context.Background(), request)
	if err == nil || result.AgentOutcome != AgentNotStarted {
		t.Fatalf("Run() = %#v, %v, want overlap rejection", result, err)
	}
	if preparer.resolveCalls != 0 || backend.openCalls != 0 || agent.prepareCalls != 0 {
		t.Fatal("overlapping roots reached resource acquisition")
	}
}

// TestRunnerRejectsPreparationProvenanceMismatch prevents candidate execution from substituting its own catalog.
func TestRunnerRejectsPreparationProvenanceMismatch(t *testing.T) {
	runner, closeLog, preparer, backend, agent := newFakeRunner(t)
	preparer.project.result.CatalogDigest = "sha256:candidate"
	result, err := runner.Run(context.Background(), fakeAttemptRequest())
	if err == nil || result.EvaluationStatus != EvaluationFixtureError {
		t.Fatalf("Run() = %#v, %v, want fixture provenance failure", result, err)
	}
	if !reflect.DeepEqual(*closeLog, []string{"project"}) || backend.openCalls != 0 || agent.prepareCalls != 0 {
		t.Fatalf("mismatch lifecycle = cleanup:%q open:%d agent:%d", *closeLog, backend.openCalls, agent.prepareCalls)
	}
}

// TestRunnerPreservesCleanupFailureBesideAgentOutcome ensures evaluator faults never rewrite what the agent did.
func TestRunnerPreservesCleanupFailureBesideAgentOutcome(t *testing.T) {
	runner, _, preparer, _, _ := newFakeRunner(t)
	preparer.project.closeErr = errors.New("cleanup failed")
	result, err := runner.Run(context.Background(), fakeAttemptRequest())
	if err == nil || !strings.Contains(err.Error(), "deferred cleanup") {
		t.Fatalf("Run() = %#v, %v, want deferred cleanup failure", result, err)
	}
	if result.AgentOutcome != AgentCompleted || result.EvaluationStatus != EvaluationEvaluatorError {
		t.Fatalf("attempt result = %#v", result)
	}
	if len(result.SecondaryFailures) != 1 || result.SecondaryFailures[0].Phase != "prepared_project" {
		t.Fatalf("secondary failures = %#v", result.SecondaryFailures)
	}
}

// TestRunnerClassifiesPostActionProviderFailureAsEvaluatorError verifies non-retryable evidence loss after the first action.
func TestRunnerClassifiesPostActionProviderFailureAsEvaluatorError(t *testing.T) {
	runner, _, _, _, agent := newFakeRunner(t)
	agent.session.waitErr = errors.New("provider disconnected")
	agent.session.result.Events = []Event{{Sequence: 1, Kind: EventMessage, Fields: map[string]string{"text": "partial provider explanation"}}}
	result, err := runner.Run(context.Background(), fakeAttemptRequest())
	if err == nil {
		t.Fatal("Run() succeeded after provider failure")
	}
	if result.AgentOutcome != AgentProviderError || result.EvaluationStatus != EvaluationEvaluatorError || !containsMilestone(result.Milestones, MilestoneFirstAgentAction) {
		t.Fatalf("attempt result = %#v", result)
	}
	transcript, readErr := os.ReadFile(filepath.Join(runner.Artifacts.root, result.AttemptID, "transcript.redacted.txt"))
	if readErr != nil || !bytes.Contains(transcript, []byte("partial provider explanation")) {
		t.Fatalf("partial diagnostic transcript = %q, %v", transcript, readErr)
	}
}

// TestRunnerRejectsInvalidSupervisorEvidenceAfterProviderFailure prevents a failed session from bypassing provenance validation.
func TestRunnerRejectsInvalidSupervisorEvidenceAfterProviderFailure(t *testing.T) {
	runner, _, _, backend, agent := newFakeRunner(t)
	agent.session.waitErr = errors.New("provider disconnected")
	backend.environment.events[0].Source = EventSourceAdapter
	result, err := runner.Run(context.Background(), fakeAttemptRequest())
	if err == nil || result.EvaluationStatus != EvaluationEvaluatorError {
		t.Fatalf("Run() = %#v, %v", result, err)
	}
	if len(result.SecondaryFailures) == 0 || result.SecondaryFailures[0].Phase != "event_capture" || !strings.Contains(result.SecondaryFailures[0].Message, "supervisor provenance") {
		t.Fatalf("secondary failures = %#v", result.SecondaryFailures)
	}
}

// TestRunnerClassifiesTimeoutAfterFirstActionAndUsesFreshCleanup verifies budgets do not poison teardown contexts.
func TestRunnerClassifiesTimeoutAfterFirstActionAndUsesFreshCleanup(t *testing.T) {
	runner, closeLog, _, _, agent := newFakeRunner(t)
	agent.session.waitForContext = true
	request := fakeAttemptRequest()
	request.Definition.Limits.WallTime = time.Millisecond
	result, err := runner.Run(context.Background(), request)
	if err == nil {
		t.Fatal("Run() succeeded after timeout")
	}
	if result.AgentOutcome != AgentTimeout || result.EvaluationStatus != EvaluationEvaluatorError {
		t.Fatalf("timeout result = %#v", result)
	}
	if !reflect.DeepEqual(*closeLog, []string{"session", "agent_preparation", "backend", "project"}) {
		t.Fatalf("cleanup order after timeout = %q", *closeLog)
	}
}

// TestRunnerClassifiesCancellationAfterFirstAction preserves operator intent without skipping cleanup.
func TestRunnerClassifiesCancellationAfterFirstAction(t *testing.T) {
	runner, closeLog, _, _, agent := newFakeRunner(t)
	agent.session.waitForContext = true
	ctx, cancel := context.WithCancel(context.Background())
	agent.session.turn.Events = []Event{{Sequence: 1, Kind: EventFileRead}}
	cancel()
	result, err := runner.Run(ctx, fakeAttemptRequest())
	if err == nil {
		t.Fatal("Run() succeeded after cancellation")
	}
	if result.AgentOutcome != AgentCancelled || result.EvaluationStatus != EvaluationEvaluatorError {
		t.Fatalf("cancelled result = %#v", result)
	}
	if !reflect.DeepEqual(*closeLog, []string{"session", "agent_preparation", "backend", "project"}) {
		t.Fatalf("cleanup order after cancellation = %q", *closeLog)
	}
}

// newFakeRunner assembles one valid deterministic lifecycle with shared cleanup recording.
func newFakeRunner(t *testing.T) (Runner, *[]string, *fakePreparer, *fakeBackend, *fakeAgent) {
	t.Helper()
	projectRoot := t.TempDir()
	sealedRoot := t.TempDir()
	closeLog := &[]string{}
	verifier := &capturingVerifier{}
	registry, err := NewRegistry(PromotedWorkflows(), []Verifier{verifier})
	if err != nil {
		t.Fatalf("NewRegistry(): %v", err)
	}
	plan := ResolvedPreparationPlan{
		ResolutionID:         "resolution-01",
		ScenarioID:           "invoice-http-route",
		ScenarioSchema:       2,
		PlanDigest:           "sha256:plan",
		ScenarioPlanDigest:   "sha256:scenario-plan",
		CatalogDigest:        "sha256:catalog",
		DependencyDigests:    map[string]string{"invoice-domain": "sha256:invoice-domain"},
		ProjectConfiguration: []byte("module_name: example.com/invoices\n"),
		ForjDigest:           "sha256:forj",
		EnvironmentDigest:    "sha256:environment",
		TargetOmitted:        true,
	}
	preparer := &fakePreparer{
		capabilities: PreparationCapabilities{ScenarioSchemaVersions: []int{2}},
		plan:         plan,
		project: &fakePreparedProject{
			result: PreparationResult{
				ResolutionID:   plan.ResolutionID,
				ProjectRoot:    projectRoot,
				ScenarioID:     plan.ScenarioID,
				ScenarioSchema: plan.ScenarioSchema,
				PlanDigest:     plan.PlanDigest,
				CatalogDigest:  plan.CatalogDigest,
				BaselineTree:   "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
				ForjExecutable: "/tools/forj",
				ForjDigest:     "sha256:forj",
			},
			closeLog: closeLog,
		},
	}
	backend := &fakeBackend{
		capabilities: append([]Capability{CapabilityCommands}, authoritativeCapabilities...),
		environment: &fakeBackendEnvironment{
			environment: RunEnvironment{ProjectRoot: projectRoot},
			baseline:    BaselineSnapshot{TreeDigest: "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", Complete: true},
			events: []Event{
				{Sequence: 1, Source: EventSourceSupervisor, Kind: EventCommandStarted, Fields: map[string]string{EventFieldCommandID: "command-1", EventFieldExecutableDigest: "sha256:forj", EventFieldArguments: `["make:controller","invoices"]`}},
				{Sequence: 2, Source: EventSourceSupervisor, Kind: EventCommandFinished, Fields: map[string]string{EventFieldCommandID: "command-1", EventFieldExitCode: "0"}},
				{Sequence: 3, Source: EventSourceSupervisor, Kind: EventRunFinished},
			},
			sealed:   SealedProject{Root: sealedRoot, TreeDigest: "sha256:final-tree"},
			closeLog: closeLog,
		},
	}
	agent := &fakeAgent{
		capabilities: []Capability{CapabilityCommands, CapabilityCredentialIsolation},
		preparation: &fakeAgentPreparation{
			agent:    PreparedAgent{Name: "fake-agent", Executable: "/tools/agent", ExecutableDigest: "sha256:agent", Model: "fake-model"},
			closeLog: closeLog,
		},
		session: &fakeSession{
			turn: AgentTurnResult{Accepted: true, Events: []Event{{Sequence: 1, Kind: EventCommandStarted, Fields: map[string]string{
				EventFieldCommandID:        "command-1",
				EventFieldExecutableDigest: "sha256:forj",
				EventFieldArguments:        `["make:controller","invoices"]`,
			}}}},
			result: AgentResult{Outcome: AgentCompleted, Events: []Event{
				{Sequence: 2, Kind: EventCommandFinished, Fields: map[string]string{EventFieldCommandID: "command-1", EventFieldExitCode: "0"}},
				{Sequence: 3, Kind: EventRunFinished},
			}},
			closeLog: closeLog,
		},
	}
	artifactStore, err := NewArtifactStore(filepath.Join(t.TempDir(), "artifacts"), []byte("0123456789abcdef0123456789abcdef"), NewRedactor(nil))
	if err != nil {
		t.Fatalf("NewArtifactStore(): %v", err)
	}
	return Runner{
		Registry:  registry,
		Preparer:  preparer,
		Backend:   backend,
		Agent:     agent,
		Guidance:  &fakeGuidanceResolver{},
		Artifacts: artifactStore,
		Now:       func() time.Time { return time.Unix(1_700_000_000, 0) },
	}, closeLog, preparer, backend, agent
}

// fakeAttemptRequest returns the exact manifest and invocation policy shared by runner tests.
func fakeAttemptRequest() AttemptRequest {
	definition, err := decodeEvaluationManifest([]byte(validEvaluationManifest))
	if err != nil {
		panic(err)
	}
	definition.Prompt = "Add an endpoint that returns an invoice by ID."
	definition.PromptDigest = "sha256:prompt"
	return AttemptRequest{
		AttemptID:      "attempt-01",
		LogicalTrialID: "trial-01",
		Definition:     definition,
		Intent:         IntentAuthoritative,
		Preparation: PreparationRequest{
			ScenarioID:      definition.ProjectScenario,
			DestinationRoot: "/private/project",
			ForjExecutable:  "/tools/forj",
			OrchestrationID: "resolution-01",
			Environment:     []string{"PATH=/tools"},
		},
		GuidanceProfile: "none",
		Runtime: RuntimeIdentity{
			Supervisor: SoftwareIdentity{Module: "github.com/goforj/atlas", Version: "v0.4.0"},
			Framework:  SoftwareIdentity{Module: "github.com/goforj/goforj", Version: "v0.25.0"},
			GoVersion:  "go1.25.0",
			GOOS:       "linux",
			GOARCH:     "amd64",
		},
	}
}
