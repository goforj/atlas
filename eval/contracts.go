// Package eval provides deterministic contracts and orchestration for Atlas live-agent evaluations.
package eval

import (
	"context"
	"fmt"
	"time"
)

// Capability identifies one observation or isolation property an evaluation component can prove.
type Capability string

const (
	// CapabilityFileReads proves agent file reads through a trusted observation boundary.
	CapabilityFileReads Capability = "file_reads"
	// CapabilityFileWrites proves agent file mutations through a trusted observation boundary.
	CapabilityFileWrites Capability = "file_writes"
	// CapabilityCommands proves process execution with trusted executable identity and arguments.
	CapabilityCommands Capability = "commands"
	// CapabilityMCPToolCalls proves MCP calls through a trusted supervisor-owned interposer.
	CapabilityMCPToolCalls Capability = "mcp_tool_calls"
	// CapabilityProcessCleanup proves complete descendant-job termination.
	CapabilityProcessCleanup Capability = "process_cleanup"
	// CapabilityCredentialIsolation proves candidate processes cannot reach reusable provider authority.
	CapabilityCredentialIsolation Capability = "credential_isolation"
	// CapabilityHostFilesystemIsolation proves candidate processes cannot read or mutate undeclared host paths.
	CapabilityHostFilesystemIsolation Capability = "host_filesystem_isolation"
	// CapabilityNetworkEnforcement proves the backend enforces the requested shell network policy.
	CapabilityNetworkEnforcement Capability = "network_enforcement"
	// CapabilityVerifierIsolation proves candidate execution cannot mutate verifier code, state, or later phases.
	CapabilityVerifierIsolation Capability = "verifier_isolation"
	// CapabilityArtifactIsolation proves candidate processes cannot read signing authority or mutate retained evidence.
	CapabilityArtifactIsolation Capability = "artifact_isolation"
)

// EvaluationDefinition is one resolved manifest and its adjacent natural-language prompt.
type EvaluationDefinition struct {
	SchemaVersion   int
	ID              string
	Summary         string
	Suite           string
	ProjectScenario string
	WorkflowID      string
	VerifierID      string
	Limits          Limits
	Prompt          string
	PromptDigest    string
}

// Limits bounds one logical evaluation trial independently from provider defaults.
type Limits struct {
	WallTime     time.Duration
	Commands     int
	ShellNetwork string
}

// RunIntent determines whether unavailable evidence blocks execution or remains an explicit diagnostic limitation.
type RunIntent string

const (
	// IntentAuthoritative requires every imported observation capability before Project mutation.
	IntentAuthoritative RunIntent = "authoritative"
	// IntentDiagnostic permits useful outcome evaluation while marking unsupported evidence endpoints ineligible.
	IntentDiagnostic RunIntent = "diagnostic"
)

// RequirementKind separates framework workflow gates from optional quality signals.
type RequirementKind string

const (
	// RequirementWorkflow is a declared framework action required for conformance.
	RequirementWorkflow RequirementKind = "workflow"
	// RequirementQuality is recorded for calibration but does not fail conformance.
	RequirementQuality RequirementKind = "quality"
)

// WorkflowRequirement defines one typed, observation-backed workflow expectation.
type WorkflowRequirement struct {
	ID          string
	Kind        RequirementKind
	Capability  Capability
	Description string
}

// GeneratorRequirement defines one exact successful GoForj generator action without coupling it to a shell spelling.
type GeneratorRequirement struct {
	ID        string
	Arguments []string
}

// WorkflowExpectation is a promoted and versioned framework workflow contract.
type WorkflowExpectation struct {
	ID           string
	Requirements []WorkflowRequirement
	Generators   []GeneratorRequirement
}

// EndpointStatus records one independently meaningful verification outcome.
type EndpointStatus string

const (
	// EndpointPassed indicates that trusted evidence satisfied the endpoint.
	EndpointPassed EndpointStatus = "passed"
	// EndpointFailed indicates that complete trusted evidence violated the endpoint.
	EndpointFailed EndpointStatus = "failed"
	// EndpointIneligible indicates that required trusted evidence was unavailable.
	EndpointIneligible EndpointStatus = "ineligible"
)

// EndpointResult describes one framework, conformance, or contract endpoint.
type EndpointResult struct {
	ID      string         `json:"id"`
	Status  EndpointStatus `json:"status"`
	Details string         `json:"details,omitempty"`
}

// VerificationInput contains sealed Project and evidence identities, never candidate verifier code.
type VerificationInput struct {
	ProjectRoot  string
	BaselineTree string
	FinalTree    string
	// Changes is the supervisor-computed, path-level projection of the sealed Project delta.
	Changes []ProjectChange
	Events  []Event
}

// ProjectPathState identifies one Project path at a sealed snapshot. A zero value means the path was absent.
type ProjectPathState struct {
	Kind   string `json:"kind,omitempty"`
	Digest string `json:"digest,omitempty"`
	Mode   uint32 `json:"mode,omitempty"`
}

// ProjectChange records one supervisor-computed path change between the treatment baseline and sealed Project.
type ProjectChange struct {
	Path   string           `json:"path"`
	Before ProjectPathState `json:"before"`
	After  ProjectPathState `json:"after"`
}

// VerificationResult separates framework behavior from workflow conformance.
type VerificationResult struct {
	FrameworkOutcome    EndpointResult   `json:"framework_outcome"`
	WorkflowConformance EndpointResult   `json:"workflow_conformance"`
	Contract            *EndpointResult  `json:"contract,omitempty"`
	Checks              []EndpointResult `json:"checks,omitempty"`
}

// Verifier is one promoted deterministic outcome contract.
type Verifier interface {
	ID() string
	Capabilities() []Capability
	Verify(context.Context, VerificationInput) (VerificationResult, error)
}

// PreparationCapabilities describe the schemas and controls a Project preparer supports.
type PreparationCapabilities struct {
	ScenarioSchemaVersions []int
}

// PreparationRequest identifies trusted scenario inputs without carrying reusable authority.
type PreparationRequest struct {
	ScenarioID      string
	DestinationRoot string
	ForjExecutable  string
	OrchestrationID string
	Environment     []string
}

// ResolvedPreparationPlan is an immutable data contract authenticated by the trusted caller.
type ResolvedPreparationPlan struct {
	ResolutionID         string
	ScenarioID           string
	ScenarioSchema       int
	PlanDigest           string
	ScenarioPlanDigest   string
	CatalogDigest        string
	ForjDigest           string
	EnvironmentDigest    string
	DependencyDigests    map[string]string
	ProjectConfiguration []byte
	TargetOmitted        bool
}

// PreparationResult records the exact Project and tool identities returned by preparation.
type PreparationResult struct {
	ResolutionID   string
	ProjectRoot    string
	ScenarioID     string
	ScenarioSchema int
	PlanDigest     string
	CatalogDigest  string
	BaselineTree   string
	ForjExecutable string
	ForjDigest     string
	OwnedPaths     []string
}

// PreparedProject owns one prepared Project until the supervisor closes it.
type PreparedProject interface {
	Result() PreparationResult
	Close(context.Context) error
}

// ProjectPreparer is the Atlas-owned boundary implemented by GoForj.
type ProjectPreparer interface {
	Capabilities(context.Context) (PreparationCapabilities, error)
	Resolve(context.Context, PreparationRequest) (ResolvedPreparationPlan, error)
	Prepare(context.Context, PreparationRequest, ResolvedPreparationPlan) (PreparedProject, error)
}

// Guidance is the exact native instructions, skills, and MCP selection installed for one treatment.
type Guidance struct {
	Profile string
	Files   map[string][]byte
	Skills  []string
	MCP     []string
}

const (
	// GuidanceProfileNone omits Project-native framework instructions for the control treatment.
	GuidanceProfileNone = "none"
	// GuidanceProfileAgents installs only the canonical Project AGENTS.md treatment.
	GuidanceProfileAgents = "agents"
)

// GuidanceResolver projects one named treatment from the prepared Project's trusted identity.
type GuidanceResolver interface {
	Resolve(context.Context, string, PreparationResult) (Guidance, error)
}

// AgentProperties declares provider-neutral safety properties enforced by an adapter.
type AgentProperties struct {
	Properties []Capability
}

// RunEnvironment is the backend-owned namespace presented to an agent adapter.
type RunEnvironment struct {
	ProjectRoot string
	HomeRoot    string
	Environment []string
}

// PreparedAgent records attributable agent identity and private configuration.
type PreparedAgent struct {
	Name             string
	Executable       string
	ExecutableDigest string
	Model            string
	Environment      RunEnvironment
}

// AgentPreparation owns private adapter resources acquired before a session starts.
type AgentPreparation interface {
	Agent() PreparedAgent
	Close(context.Context) error
}

// AgentTurn is one supervisor-selected natural-language interaction.
type AgentTurn struct {
	Prompt string
	Limits Limits
}

// AgentTurnResult records prompt acceptance and any events emitted before it returned.
type AgentTurnResult struct {
	Accepted bool
	Events   []Event
}

// AgentResult records provider completion independently from evaluator validity.
type AgentResult struct {
	Outcome AgentOutcome
	Events  []Event
	Message string
}

// AgentSessionIdentity records effective provider identity established only after a fresh session starts.
type AgentSessionIdentity struct {
	Version       string
	Model         string
	ModelProvider string
}

// EvaluationSession owns one fresh provider session and its complete descendant job.
type EvaluationSession interface {
	Identity() AgentSessionIdentity
	Turn(context.Context, AgentTurn) (AgentTurnResult, error)
	Wait(context.Context) (AgentResult, error)
	Close(context.Context) error
}

// EvaluationAgent prepares and starts fresh non-interactive evaluation sessions.
type EvaluationAgent interface {
	Name() string
	Properties(context.Context) (AgentProperties, error)
	Prepare(context.Context, RunEnvironment, Guidance) (AgentPreparation, error)
	Start(context.Context, PreparedAgent) (EvaluationSession, error)
}

// BackendEnvironment owns the isolation resources that enclose one agent attempt.
type BackendEnvironment interface {
	Environment() RunEnvironment
	Baseline(context.Context) (BaselineSnapshot, error)
	ObservedEvents(context.Context) ([]Event, error)
	Seal(context.Context) (SealedProject, error)
	Close(context.Context) error
}

// BaselineSnapshot is the backend/supervisor-owned baseline captured after treatment setup and before the session starts.
type BaselineSnapshot struct {
	TreeDigest string
	Complete   bool
}

// SealedProject is the immutable verifier input captured after every agent descendant has stopped.
type SealedProject struct {
	Root       string
	TreeDigest string
}

// BackendRequest binds one prepared Project to explicit execution policy.
type BackendRequest struct {
	Project      PreparedProject
	ShellNetwork string
	Environment  []string
	CommandLimit int
}

// ExecutionBackend creates one private execution boundary and reports what it can prove.
type ExecutionBackend interface {
	Name() string
	Capabilities(context.Context) ([]Capability, error)
	Open(context.Context, BackendRequest) (BackendEnvironment, error)
}

// EventKind is the provider-neutral class of one observed agent action.
type EventKind string

const (
	// EventFileRead records one observed Project read.
	EventFileRead EventKind = "file_read"
	// EventFileWrite records one observed Project mutation.
	EventFileWrite EventKind = "file_write"
	// EventCommandStarted records trusted process execution before completion.
	EventCommandStarted EventKind = "command_started"
	// EventCommandFinished records trusted process completion.
	EventCommandFinished EventKind = "command_finished"
	// EventMCPToolCalled records one trusted MCP request and result classification.
	EventMCPToolCalled EventKind = "mcp_tool_called"
	// EventMessage records inert provider text after redaction.
	EventMessage EventKind = "message"
	// EventRunFinished records agent session completion.
	EventRunFinished EventKind = "run_finished"
)

// Event is one ordered provider-neutral observation.
type Event struct {
	Sequence uint64            `json:"sequence"`
	Kind     EventKind         `json:"kind"`
	Source   EventSource       `json:"source"`
	Time     time.Time         `json:"time"`
	Fields   map[string]string `json:"fields,omitempty"`
}

// EventSource identifies the observation boundary that produced an event.
type EventSource string

const (
	// EventSourceSupervisor identifies evidence observed by the execution backend or supervisor.
	EventSourceSupervisor EventSource = "supervisor"
	// EventSourceAdapter identifies diagnostic-only telemetry reported by an agent adapter.
	EventSourceAdapter EventSource = "adapter"
)

const (
	// EventFieldCommandID correlates one trusted command start with its completion.
	EventFieldCommandID = "command_id"
	// EventFieldExecutableDigest identifies the resolved executable observed by the supervisor.
	EventFieldExecutableDigest = "executable_digest"
	// EventFieldArguments carries a JSON array of arguments excluding the executable.
	EventFieldArguments = "arguments"
	// EventFieldExitCode records the decimal process exit code for a completed command.
	EventFieldExitCode = "exit_code"
)

// AgentOutcome classifies what happened to the provider-side attempt.
type AgentOutcome string

const (
	// AgentNotStarted indicates preflight ended before a provider session began.
	AgentNotStarted AgentOutcome = "not_started"
	// AgentCompleted indicates the agent reached a terminal response or process exit.
	AgentCompleted AgentOutcome = "completed"
	// AgentAbstained indicates an accepted safe clarification ended the attempt.
	AgentAbstained AgentOutcome = "abstained"
	// AgentProviderError indicates the provider failed independently from the adapter.
	AgentProviderError AgentOutcome = "provider_error"
	// AgentAdapterError indicates the adapter failed independently from the provider.
	AgentAdapterError AgentOutcome = "adapter_error"
	// AgentTimeout indicates the attempt exceeded its wall-time budget.
	AgentTimeout AgentOutcome = "timeout"
	// AgentCancelled indicates an operator cancelled the attempt.
	AgentCancelled AgentOutcome = "cancelled"
)

// AgentFailure classifies a failed adapter operation without conflating provider availability with adapter defects.
type AgentFailure struct {
	Outcome AgentOutcome
	Err     error
}

// Error returns the wrapped operation failure.
func (failure *AgentFailure) Error() string {
	if failure == nil || failure.Err == nil {
		return "agent operation failed"
	}
	return failure.Err.Error()
}

// Unwrap exposes the original operation failure.
func (failure *AgentFailure) Unwrap() error {
	if failure == nil {
		return nil
	}
	return failure.Err
}

// EvaluationStatus classifies whether trusted verification produced a valid result.
type EvaluationStatus string

const (
	// EvaluationValid indicates complete evidence produced ordinary endpoints.
	EvaluationValid EvaluationStatus = "valid"
	// EvaluationValidAbstention indicates the scenario accepted a safe abstention.
	EvaluationValidAbstention EvaluationStatus = "valid_abstention"
	// EvaluationNotEvaluated indicates no evidence-valid logical trial began.
	EvaluationNotEvaluated EvaluationStatus = "not_evaluated"
	// EvaluationIneligible indicates a required capability was unavailable at preflight.
	EvaluationIneligible EvaluationStatus = "ineligible"
	// EvaluationDiagnostic indicates diagnostic artifacts were collected without the evidence required for a valid evaluation.
	EvaluationDiagnostic EvaluationStatus = "diagnostic"
	// EvaluationFixtureError indicates Project preparation or starting-state verification failed.
	EvaluationFixtureError EvaluationStatus = "fixture_error"
	// EvaluationEvaluatorError indicates capture, verification, or cleanup became unreliable.
	EvaluationEvaluatorError EvaluationStatus = "evaluator_error"
)

// Milestone is one monotonic trusted lifecycle boundary.
type Milestone string

const (
	// MilestonePreflight records successful contract and capability resolution.
	MilestonePreflight Milestone = "preflight"
	// MilestoneProviderSessionStarted records acquisition of a provider session.
	MilestoneProviderSessionStarted Milestone = "provider_session_started"
	// MilestonePromptDelivered records successful prompt submission.
	MilestonePromptDelivered Milestone = "prompt_delivered"
	// MilestoneFirstAgentAction records the first trusted action or terminal response.
	MilestoneFirstAgentAction Milestone = "first_agent_action"
	// MilestoneAgentTerminal records provider-side terminal completion.
	MilestoneAgentTerminal Milestone = "agent_terminal"
	// MilestoneEvaluationTerminal records verifier and cleanup completion.
	MilestoneEvaluationTerminal Milestone = "evaluation_terminal"
)

// SecondaryFailure preserves evaluator failures without overwriting the agent outcome.
type SecondaryFailure struct {
	Phase   string `json:"phase"`
	Message string `json:"message"`
}

// AttemptResult is the complete lifecycle result for one stochastic attempt.
type AttemptResult struct {
	AttemptID           string                 `json:"attempt_id"`
	LogicalTrialID      string                 `json:"logical_trial_id"`
	EvaluationID        string                 `json:"evaluation_id"`
	PromptDigest        string                 `json:"prompt_digest"`
	GuidanceProfile     string                 `json:"guidance_profile"`
	GuidanceDigest      string                 `json:"guidance_digest,omitempty"`
	GuidanceFiles       []GuidanceFileIdentity `json:"guidance_files,omitempty"`
	ScenarioID          string                 `json:"scenario_id"`
	ScenarioSchema      int                    `json:"scenario_schema,omitempty"`
	PlanDigest          string                 `json:"plan_digest,omitempty"`
	ScenarioPlanDigest  string                 `json:"scenario_plan_digest,omitempty"`
	CatalogDigest       string                 `json:"catalog_digest,omitempty"`
	DependencyDigests   map[string]string      `json:"dependency_digests,omitempty"`
	ProjectConfigDigest string                 `json:"project_config_digest,omitempty"`
	EnvironmentDigest   string                 `json:"environment_digest,omitempty"`
	PreparedTree        string                 `json:"prepared_tree,omitempty"`
	BaselineTree        string                 `json:"baseline_tree,omitempty"`
	FinalTree           string                 `json:"final_tree,omitempty"`
	ForjExecutable      string                 `json:"forj_executable,omitempty"`
	ForjDigest          string                 `json:"forj_digest,omitempty"`
	Backend             string                 `json:"backend,omitempty"`
	Agent               string                 `json:"agent,omitempty"`
	AgentExecutable     string                 `json:"agent_executable,omitempty"`
	AgentDigest         string                 `json:"agent_digest,omitempty"`
	AgentVersion        string                 `json:"agent_version,omitempty"`
	Model               string                 `json:"model,omitempty"`
	ModelProvider       string                 `json:"model_provider,omitempty"`
	Runtime             RuntimeIdentity        `json:"runtime"`
	AgentOutcome        AgentOutcome           `json:"agent_outcome"`
	EvaluationStatus    EvaluationStatus       `json:"evaluation_status"`
	Milestones          []Milestone            `json:"milestones"`
	Verification        *VerificationResult    `json:"verification,omitempty"`
	SecondaryFailures   []SecondaryFailure     `json:"secondary_failures,omitempty"`
	UnavailableEvidence []Capability           `json:"unavailable_evidence,omitempty"`
	StartedAt           time.Time              `json:"started_at"`
	FinishedAt          time.Time              `json:"finished_at"`
}

// SoftwareIdentity records a retrievable release or development revision without relying on a temporary executable path.
type SoftwareIdentity struct {
	Module  string `json:"module"`
	Version string `json:"version"`
	Commit  string `json:"commit,omitempty"`
	Dirty   bool   `json:"dirty,omitempty"`
}

// RuntimeIdentity records the supervisor, framework, and Go runtime needed to reconstruct a diagnostic environment.
type RuntimeIdentity struct {
	Supervisor SoftwareIdentity `json:"supervisor"`
	Framework  SoftwareIdentity `json:"framework"`
	GoVersion  string           `json:"go_version"`
	GOOS       string           `json:"goos"`
	GOARCH     string           `json:"goarch"`
}

// GuidanceFileIdentity records one native instruction projection without retaining its content in run metadata.
type GuidanceFileIdentity struct {
	Path   string `json:"path"`
	Digest string `json:"digest"`
}

// TriageState records whether a human has established the cause of a failed attempt.
type TriageState string

const (
	// TriageUnreviewed indicates that no product disposition has been confirmed.
	TriageUnreviewed TriageState = "unreviewed"
	// TriageNeedsEvidence indicates that retained evidence cannot yet support a disposition.
	TriageNeedsEvidence TriageState = "needs-evidence"
)

// TriageRecord keeps automated suspicion distinct from a confirmed product cause.
type TriageRecord struct {
	State          TriageState `json:"state"`
	SuspectedCause string      `json:"suspected_cause,omitempty"`
	Confidence     string      `json:"confidence,omitempty"`
	EvidenceNeeded []string    `json:"evidence_needed,omitempty"`
}

// TriageDisposition is a human-confirmed product cause kept outside immutable attempt artifacts.
type TriageDisposition string

// TriageReview associates a later human disposition with an authenticated attempt without rewriting its artifacts.
type TriageReview struct {
	AttemptID   string            `json:"attempt_id"`
	Disposition TriageDisposition `json:"disposition"`
	Reviewer    string            `json:"reviewer"`
	ReviewedAt  time.Time         `json:"reviewed_at"`
}

// NewTriageReview creates an external review record; callers must store it in their review system rather than mutating signed attempt artifacts.
func NewTriageReview(attemptID string, disposition TriageDisposition, reviewer string, reviewedAt time.Time) (TriageReview, error) {
	if attemptID == "" || disposition == "" || reviewer == "" || reviewedAt.IsZero() {
		return TriageReview{}, fmt.Errorf("triage review requires an attempt ID, disposition, reviewer, and timestamp")
	}
	return TriageReview{AttemptID: attemptID, Disposition: disposition, Reviewer: reviewer, ReviewedAt: reviewedAt.UTC()}, nil
}
