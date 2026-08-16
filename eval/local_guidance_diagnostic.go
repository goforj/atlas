package eval

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LocalGuidanceDiagnostic owns Atlas's standard paired local diagnostic wiring.
// Hosts retain their Project preparation, private environments, runtime identity, and filesystem lifecycle.
type LocalGuidanceDiagnostic struct {
	runner         Runner
	forjExecutable string
	runtime        RuntimeIdentity
}

// LocalGuidanceDiagnosticOptions supplies host-owned boundaries to the standard local diagnostic service.
type LocalGuidanceDiagnosticOptions struct {
	WorkRoot            string
	ArtifactRoot        string
	ArtifactKey         []byte
	Redactor            Redactor
	Preparer            ProjectPreparer
	Codex               CodexOptions
	GoExecutable        string
	ForjExecutable      string
	VerifierEnvironment []string
	Runtime             RuntimeIdentity
}

// LocalGuidanceDiagnosticRequest identifies one paired treatment while keeping host-private environments outside Atlas policy wiring.
type LocalGuidanceDiagnosticRequest struct {
	EvaluationID    string
	DestinationRoot string
	Environments    map[string][]string
	LogicalTrialID  string
}

// NewLocalGuidanceDiagnostic creates the Atlas-owned registry, promoted verifier, Codex adapter, unconfined backend, artifact store, and runner.
func NewLocalGuidanceDiagnostic(options LocalGuidanceDiagnosticOptions) (*LocalGuidanceDiagnostic, error) {
	if strings.TrimSpace(options.WorkRoot) == "" {
		return nil, fmt.Errorf("local diagnostic work root is required")
	}
	if strings.TrimSpace(options.ArtifactRoot) == "" {
		return nil, fmt.Errorf("local diagnostic artifact root is required")
	}
	if options.Preparer == nil {
		return nil, fmt.Errorf("local diagnostic Project preparer is required")
	}
	if strings.TrimSpace(options.GoExecutable) == "" || strings.TrimSpace(options.ForjExecutable) == "" {
		return nil, fmt.Errorf("local diagnostic Go and Forj executables are required")
	}
	if err := os.MkdirAll(filepath.Join(options.WorkRoot, "verifier"), 0o700); err != nil {
		return nil, fmt.Errorf("create local diagnostic verifier work root: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(options.WorkRoot, "backend"), 0o700); err != nil {
		return nil, fmt.Errorf("create local diagnostic backend work root: %w", err)
	}
	verifierCommands := VerifierCommands{
		WorkRoot:       filepath.Join(options.WorkRoot, "verifier"),
		GoExecutable:   options.GoExecutable,
		ForjExecutable: options.ForjExecutable,
		Environment:    append([]string(nil), options.VerifierEnvironment...),
	}
	registry, err := NewRegistry(PromotedWorkflows(), PromotedVerifiers(verifierCommands))
	if err != nil {
		return nil, err
	}
	agent, err := NewCodexAgent(options.Codex)
	if err != nil {
		return nil, err
	}
	artifacts, err := NewArtifactStore(options.ArtifactRoot, options.ArtifactKey, agent.credential.Redactor(options.Redactor))
	if err != nil {
		return nil, err
	}
	return &LocalGuidanceDiagnostic{
		runner: Runner{
			Registry:  registry,
			Preparer:  options.Preparer,
			Backend:   UnconfinedLocal{WorkRoot: filepath.Join(options.WorkRoot, "backend")},
			Agent:     agent,
			Guidance:  ProjectGuidanceResolver{},
			Artifacts: artifacts,
		},
		forjExecutable: options.ForjExecutable,
		runtime:        options.Runtime,
	}, nil
}

// Run evaluates the promoted definition with fresh no-guidance and AGENTS.md treatments.
func (diagnostic *LocalGuidanceDiagnostic) Run(ctx context.Context, request LocalGuidanceDiagnosticRequest) (GuidanceDiagnosticResult, error) {
	if diagnostic == nil {
		return GuidanceDiagnosticResult{}, fmt.Errorf("local guidance diagnostic is required")
	}
	definition, err := LoadPromotedDefinition(request.EvaluationID)
	if err != nil {
		return GuidanceDiagnosticResult{}, err
	}
	trialID := request.LogicalTrialID
	if strings.TrimSpace(trialID) == "" {
		trialID, err = newLocalDiagnosticTrialID()
		if err != nil {
			return GuidanceDiagnosticResult{}, err
		}
	}
	return diagnostic.runner.RunGuidanceDiagnostic(ctx, GuidanceDiagnosticRequest{
		LogicalTrialID:  trialID,
		Definition:      definition,
		DestinationRoot: request.DestinationRoot,
		ForjExecutable:  diagnostic.forjExecutable,
		Environments:    request.Environments,
		Runtime:         diagnostic.runtime,
	})
}

// newLocalDiagnosticTrialID makes a sortable opaque identifier without treating wall-clock time as unique.
func newLocalDiagnosticTrialID() (string, error) {
	random := make([]byte, 6)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	return fmt.Sprintf("trial-%s-%s", time.Now().UTC().Format("20060102t150405"), hex.EncodeToString(random)), nil
}
