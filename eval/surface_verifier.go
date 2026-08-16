package eval

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// surfaceContract describes one framework surface without turning evaluation manifests into executable policy.
type surfaceContract struct {
	id             string
	allowedChanges []string
	sources        []sourceContract
	forbiddenText  []textExclusion
	commands       []commandContract
}

// textExclusion rejects one semantic leak into a protected Project surface.
type textExclusion struct {
	id    string
	paths []string
	text  string
}

// sourceContract requires syntax-bearing facts from one or more candidate-owned files.
type sourceContract struct {
	id             string
	paths          []string
	identifiers    []string
	selectorCalls  []string
	forbiddenCalls []string
	stringLiterals []string
	text           []string
}

// commandContract defines one supervisor-owned executable check.
type commandContract struct {
	id        string
	arguments []string
	contains  string
}

// SurfaceVerifier verifies one promoted framework surface through syntax, ownership, and isolated commands.
type SurfaceVerifier struct {
	runner   CommandRunner
	contract surfaceContract
}

// NewSurfaceVerifier creates a verifier from a reviewed in-code contract.
func NewSurfaceVerifier(runner CommandRunner, contract surfaceContract) *SurfaceVerifier {
	return &SurfaceVerifier{runner: runner, contract: contract}
}

// ID returns the promoted verifier contract identity.
func (verifier *SurfaceVerifier) ID() string {
	return verifier.contract.id
}

// Capabilities returns no agent-observation requirements because checks consume sealed candidate evidence.
func (*SurfaceVerifier) Capabilities() []Capability {
	return nil
}

// Verify checks semantic source facts, change ownership, and isolated executable behavior.
func (verifier *SurfaceVerifier) Verify(ctx context.Context, input VerificationInput) (VerificationResult, error) {
	if verifier == nil || verifier.runner == nil {
		return VerificationResult{}, fmt.Errorf("surface verifier requires an isolated command runner")
	}
	checks := []EndpointResult{verifySurfaceOwnership(input.Changes, verifier.contract.allowedChanges)}
	for _, contract := range verifier.contract.sources {
		checks = append(checks, verifySurfaceSource(input.ProjectRoot, contract))
	}
	for _, exclusion := range verifier.contract.forbiddenText {
		checks = append(checks, verifySurfaceTextAbsent(input.ProjectRoot, exclusion))
	}
	if surfaceChecksFailed(checks) {
		return VerificationResult{
			FrameworkOutcome:    summarizeSurfaceChecks(verifier.ID(), checks),
			WorkflowConformance: EndpointResult{ID: "workflow-owned-by-runner", Status: EndpointIneligible},
			Checks:              checks,
		}, nil
	}
	for _, contract := range verifier.contract.commands {
		checks = append(checks, runIsolatedCommand(ctx, verifier.runner, input.ProjectRoot, contract.id, contract.arguments, contract.contains))
	}
	framework := summarizeSurfaceChecks(verifier.ID(), checks)
	return VerificationResult{
		FrameworkOutcome:    framework,
		WorkflowConformance: EndpointResult{ID: "workflow-owned-by-runner", Status: EndpointIneligible},
		Checks:              checks,
	}, nil
}

// surfaceChecksFailed avoids executing candidate code after sealed static evidence already proves failure.
func surfaceChecksFailed(checks []EndpointResult) bool {
	for _, check := range checks {
		if check.Status == EndpointFailed {
			return true
		}
	}
	return false
}

// verifySurfaceTextAbsent protects an owning App or generated boundary from cross-surface registration.
func verifySurfaceTextAbsent(root string, exclusion textExclusion) EndpointResult {
	paths, err := matchingSurfacePaths(root, exclusion.paths)
	if err != nil {
		return EndpointResult{ID: exclusion.id, Status: EndpointFailed, Details: err.Error()}
	}
	for _, path := range paths {
		body, err := os.ReadFile(path)
		if err != nil {
			return EndpointResult{ID: exclusion.id, Status: EndpointFailed, Details: err.Error()}
		}
		if strings.Contains(string(body), exclusion.text) {
			return EndpointResult{ID: exclusion.id, Status: EndpointFailed, Details: fmt.Sprintf("protected path %q contains %q", filepath.ToSlash(path), exclusion.text)}
		}
	}
	return EndpointResult{ID: exclusion.id, Status: EndpointPassed}
}

// verifySurfaceOwnership limits candidate changes while allowing regenerated Wire output as derived evidence.
func verifySurfaceOwnership(changes []ProjectChange, patterns []string) EndpointResult {
	var unrelated []string
	for _, change := range changes {
		path := filepath.ToSlash(change.Path)
		if derivedSurfaceChange(path) {
			continue
		}
		allowed := false
		for _, pattern := range patterns {
			matched, _ := filepath.Match(pattern, path)
			ownsDescendant := (change.After.Kind == "directory" || change.Before.Kind == "directory") && strings.HasPrefix(pattern, path+"/")
			if matched || ownsDescendant || strings.HasSuffix(pattern, "/**") && strings.HasPrefix(path, strings.TrimSuffix(pattern, "**")) {
				allowed = true
				break
			}
		}
		if !allowed {
			unrelated = append(unrelated, path)
		}
	}
	if len(unrelated) > 0 {
		return EndpointResult{ID: "change-ownership", Status: EndpointFailed, Details: fmt.Sprintf("candidate changed unrelated paths: %s", strings.Join(unrelated, ", "))}
	}
	return EndpointResult{ID: "change-ownership", Status: EndpointPassed}
}

// derivedSurfaceChange identifies framework and Go tool outputs that are verified through isolated commands rather than authored ownership.
func derivedSurfaceChange(path string) bool {
	if path == "go.sum" || filepath.Base(path) == "wire_gen.go" {
		return true
	}
	if path == "_data" || strings.Contains(path, "/_data/") || strings.HasSuffix(path, "/_data") {
		return true
	}
	return path == "bin" || path == "build" || strings.HasPrefix(path, "bin/") || strings.HasPrefix(path, "build/")
}

// verifySurfaceSource finds syntax-bearing facts without counting comments as implementation evidence.
func verifySurfaceSource(root string, contract sourceContract) EndpointResult {
	paths, err := matchingSurfacePaths(root, contract.paths)
	if err != nil {
		return EndpointResult{ID: contract.id, Status: EndpointFailed, Details: err.Error()}
	}
	facts := sourceFacts{identifiers: map[string]bool{}, selectorCalls: map[string]bool{}, stringLiterals: map[string]bool{}}
	var text strings.Builder
	for _, path := range paths {
		body, err := os.ReadFile(path)
		if err != nil {
			return EndpointResult{ID: contract.id, Status: EndpointFailed, Details: err.Error()}
		}
		text.Write(body)
		if filepath.Ext(path) != ".go" {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, body, 0)
		if err != nil {
			return EndpointResult{ID: contract.id, Status: EndpointFailed, Details: fmt.Sprintf("parse %s: %v", filepath.ToSlash(path), err)}
		}
		collectSourceFacts(file, &facts)
	}
	for _, identifier := range contract.identifiers {
		if !facts.identifiers[identifier] {
			return EndpointResult{ID: contract.id, Status: EndpointFailed, Details: fmt.Sprintf("required identifier %q is absent", identifier)}
		}
	}
	for _, selector := range contract.selectorCalls {
		if !facts.selectorCalls[selector] {
			return EndpointResult{ID: contract.id, Status: EndpointFailed, Details: fmt.Sprintf("required call %q is absent", selector)}
		}
	}
	for _, selector := range contract.forbiddenCalls {
		if facts.selectorCalls[selector] {
			return EndpointResult{ID: contract.id, Status: EndpointFailed, Details: fmt.Sprintf("forbidden call %q is present", selector)}
		}
	}
	for _, literal := range contract.stringLiterals {
		if !facts.stringLiterals[literal] {
			return EndpointResult{ID: contract.id, Status: EndpointFailed, Details: fmt.Sprintf("required string literal %q is absent", literal)}
		}
	}
	for _, required := range contract.text {
		if !strings.Contains(text.String(), required) {
			return EndpointResult{ID: contract.id, Status: EndpointFailed, Details: fmt.Sprintf("required configuration %q is absent", required)}
		}
	}
	return EndpointResult{ID: contract.id, Status: EndpointPassed}
}

// sourceFacts retains the syntax classes used by promoted surface contracts.
type sourceFacts struct {
	identifiers    map[string]bool
	selectorCalls  map[string]bool
	stringLiterals map[string]bool
}

// collectSourceFacts records declarations and calls while excluding comments from evidence.
func collectSourceFacts(file *ast.File, facts *sourceFacts) {
	ast.Inspect(file, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.Ident:
			facts.identifiers[value.Name] = true
		case *ast.CallExpr:
			if selector, ok := value.Fun.(*ast.SelectorExpr); ok {
				facts.selectorCalls[expressionTypeName(selector.X)+"."+selector.Sel.Name] = true
				facts.selectorCalls[selector.Sel.Name] = true
			}
		case *ast.BasicLit:
			if value.Kind == token.STRING {
				facts.stringLiterals[strings.Trim(value.Value, "`\"")] = true
			}
		}
		return true
	})
}

// matchingSurfacePaths resolves reviewed Project-relative globs without following arbitrary external paths.
func matchingSurfacePaths(root string, patterns []string) ([]string, error) {
	set := map[string]bool{}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(pattern)))
		if err != nil {
			return nil, err
		}
		for _, match := range matches {
			set[match] = true
		}
	}
	paths := make([]string, 0, len(set))
	for path := range set {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return nil, fmt.Errorf("none of the required paths exist: %s", strings.Join(patterns, ", "))
	}
	return paths, nil
}

// runIsolatedCommand executes one check in a private clone and always destroys its writable state.
func runIsolatedCommand(ctx context.Context, runner CommandRunner, root, id string, command []string, contains string) (result EndpointResult) {
	session, err := runner.Open(ctx, root)
	if err != nil {
		return EndpointResult{ID: id, Status: EndpointFailed, Details: fmt.Sprintf("open isolated verifier session: %v", err)}
	}
	defer func() {
		cleanupContext, cancel := context.WithTimeout(context.Background(), verifierCleanupTimeout)
		defer cancel()
		if closeErr := session.Close(cleanupContext); closeErr != nil && result.Status == EndpointPassed {
			result = EndpointResult{ID: id, Status: EndpointFailed, Details: fmt.Sprintf("close isolated verifier session: %v", closeErr)}
		}
	}()
	return runCheck(ctx, session, id, command, contains)
}

// summarizeSurfaceChecks produces one endpoint without concealing individual failures.
func summarizeSurfaceChecks(id string, checks []EndpointResult) EndpointResult {
	failed := 0
	ineligible := 0
	for _, check := range checks {
		switch check.Status {
		case EndpointFailed:
			failed++
		case EndpointIneligible:
			ineligible++
		}
	}
	if failed > 0 {
		return EndpointResult{ID: id, Status: EndpointFailed, Details: fmt.Sprintf("%d of %d framework checks failed", failed, len(checks))}
	}
	if ineligible > 0 {
		return EndpointResult{ID: id, Status: EndpointIneligible, Details: fmt.Sprintf("%d of %d framework checks lack authoritative evidence", ineligible, len(checks))}
	}
	return EndpointResult{ID: id, Status: EndpointPassed}
}
