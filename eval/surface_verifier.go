package eval

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

const maxWireOutputFiles = 64

// surfaceContract describes one framework surface without turning evaluation manifests into executable policy.
type surfaceContract struct {
	id                     string
	allowedChanges         []string
	requiredChanges        []string
	qualityTestPatterns    []string
	baselineTestExclusions []string
	sources                []sourceContract
	forbiddenText          []textExclusion
	commands               []commandContract
}

// textExclusion rejects one semantic leak into a protected Project surface.
type textExclusion struct {
	id    string
	paths []string
	text  string
}

// sourceContract requires syntax-bearing facts from one or more candidate-owned files.
type sourceContract struct {
	id                   string
	paths                []string
	identifiers          []string
	identifierChoices    [][]string
	selectorCalls        []string
	forbiddenCalls       []string
	stringLiterals       []string
	forbiddenLiterals    []string
	declarations         []declarationContract
	assignments          []assignmentContract
	routeGroups          []routeGroupContract
	text                 []string
	normalizedText       []string
	appConfiguration     *appConfigurationContract
	commentOnly          bool
	sqlColumnChanges     []sqlColumnChangeContract
	providerConnection   *providerConnectionContract
	scheduleRegistration bool
}

// appConfigurationContract requires an App's persisted Project configuration rather than its development watcher settings.
type appConfigurationContract struct {
	name               string
	requiredComponents []string
}

// sqlColumnChangeContract requires one SQL migration to add or remove a named table column.
type sqlColumnChangeContract struct {
	table  string
	column string
	add    bool
}

// providerConnectionContract requires an accessor-using provider to be registered in an App Wire set.
type providerConnectionContract struct {
	accessor            string
	managerImportSuffix string
	wirePaths           []string
}

// assignmentContract relates a named local value to the calls and identifiers used to build it.
type assignmentContract struct {
	name                 string
	identifiers          []string
	forbiddenIdentifiers []string
	selectorCalls        []string
}

// routeGroupContract requires one route group to join the intended route slice and middleware.
type routeGroupContract struct {
	routesIdentifier   string
	middlewareSelector string
}

// declarationContract requires related syntax to occur inside one named declaration instead of anywhere in a package.
type declarationContract struct {
	name                 string
	nameChoices          []string
	anyName              bool
	receiver             string
	identifiers          []string
	forbiddenIdentifiers []string
	selectorCalls        []string
	forbiddenCalls       []string
	stringLiterals       []string
	forbiddenLiterals    []string
	nestedCalls          []nestedCallContract
	argumentFlows        []callArgumentFlowContract
}

// nestedCallContract proves an inner call occurs within an outer call expression, including callback arguments.
type nestedCallContract struct {
	outer string
	inner string
}

// callArgumentFlowContract requires a call argument to originate from an accessor call containing a configured literal.
type callArgumentFlowContract struct {
	call    string
	literal string
}

// commandContract defines one supervisor-owned executable check.
type commandContract struct {
	id                 string
	arguments          []string
	contains           []string
	supervisorFiles    []supervisorFile
	probe              func(context.Context, CommandRunner, VerifierProject) EndpointResult
	standard           bool
	standardBuilds     [][]string
	namedResourceProbe *providerConnectionContract
}

// readableCommandSession exposes candidate-derived files from a verifier-owned clone without expanding the public command-session contract.
type readableCommandSession interface {
	ReadFile(string) ([]byte, error)
	FilesNamed(string) ([]string, error)
}

// verifyWireOutputParity proves checked-in Wire output is the result of the supported full build path.
func verifyWireOutputParity() func(context.Context, CommandRunner, VerifierProject) EndpointResult {
	return func(ctx context.Context, runner CommandRunner, project VerifierProject) (result EndpointResult) {
		session, err := runner.Open(ctx, project)
		if err != nil {
			return EndpointResult{ID: "wire-output-parity", Status: EndpointFailed, Details: fmt.Sprintf("open isolated verifier session: %v", err)}
		}
		defer func() {
			cleanupContext, cancel := context.WithTimeout(context.Background(), verifierCleanupTimeout)
			defer cancel()
			if closeErr := session.Close(cleanupContext); closeErr != nil && result.Status == EndpointPassed {
				result = EndpointResult{ID: "wire-output-parity", Status: EndpointFailed, Details: fmt.Sprintf("close isolated verifier session: %v", closeErr)}
			}
		}()
		return verifyWireOutputParityInSession(ctx, session, defaultWireBuildCommands())
	}
}

// verifyWireOutputParityInSession keeps regeneration and compilation in one private clone so the latter can reuse the former's build cache.
func verifyWireOutputParityInSession(ctx context.Context, session CommandSession, buildCommands [][]string) EndpointResult {
	reader, ok := session.(readableCommandSession)
	if !ok {
		return EndpointResult{ID: "wire-output-parity", Status: EndpointFailed, Details: "isolated verifier session cannot read regenerated Wire output"}
	}
	beforePaths, listErr := reader.FilesNamed("wire_gen.go")
	if listErr != nil {
		return EndpointResult{ID: "wire-output-parity", Status: EndpointFailed, Details: fmt.Sprintf("list checked-in Wire output: %v", listErr)}
	}
	if len(beforePaths) > maxWireOutputFiles {
		return EndpointResult{ID: "wire-output-parity", Status: EndpointFailed, Details: fmt.Sprintf("checked-in Wire output exceeds %d files", maxWireOutputFiles)}
	}
	before := make(map[string][sha256.Size]byte, len(beforePaths))
	for _, path := range beforePaths {
		body, readErr := reader.ReadFile(path)
		if readErr != nil {
			return EndpointResult{ID: "wire-output-parity", Status: EndpointFailed, Details: fmt.Sprintf("read checked-in Wire output %q: %v", path, readErr)}
		}
		before[path] = sha256.Sum256(body)
	}
	for _, command := range buildCommands {
		if _, err := session.Run(ctx, command); err != nil {
			return EndpointResult{ID: "wire-output-parity", Status: EndpointFailed, Details: fmt.Sprintf("regenerate Wire through supported build path %q: %v", strings.Join(command, " "), err)}
		}
	}
	afterPaths, listErr := reader.FilesNamed("wire_gen.go")
	if listErr != nil {
		return EndpointResult{ID: "wire-output-parity", Status: EndpointFailed, Details: fmt.Sprintf("list regenerated Wire output: %v", listErr)}
	}
	if !slices.Equal(beforePaths, afterPaths) {
		return EndpointResult{ID: "wire-output-parity", Status: EndpointFailed, Details: "checked-in Wire output paths differ from supported regeneration"}
	}
	for _, path := range beforePaths {
		after, readErr := reader.ReadFile(path)
		if readErr != nil {
			return EndpointResult{ID: "wire-output-parity", Status: EndpointFailed, Details: fmt.Sprintf("read regenerated Wire output %q: %v", path, readErr)}
		}
		if before[path] != sha256.Sum256(after) {
			return EndpointResult{ID: "wire-output-parity", Status: EndpointFailed, Details: fmt.Sprintf("checked-in Wire output %q differs from supported regeneration", path)}
		}
	}
	return EndpointResult{ID: "wire-output-parity", Status: EndpointPassed}
}

// supervisorFile installs verifier-owned executable evidence after candidate tests have been removed.
type supervisorFile struct {
	path string
	body string
}

// surfaceVerifier verifies one promoted framework surface through syntax, ownership, and isolated commands.
type surfaceVerifier struct {
	runner   CommandRunner
	contract surfaceContract
}

// newSurfaceVerifier creates a verifier from a reviewed in-code contract.
func newSurfaceVerifier(runner CommandRunner, contract surfaceContract) *surfaceVerifier {
	return &surfaceVerifier{runner: runner, contract: contract}
}

// ID returns the promoted verifier contract identity.
func (verifier *surfaceVerifier) ID() string {
	return verifier.contract.id
}

// Capabilities returns no agent-observation requirements because checks consume sealed candidate evidence.
func (*surfaceVerifier) Capabilities() []Capability {
	return nil
}

// Verify checks semantic source facts, change ownership, and isolated executable behavior.
func (verifier *surfaceVerifier) Verify(ctx context.Context, input VerificationInput) (VerificationResult, error) {
	if verifier == nil || verifier.runner == nil {
		return VerificationResult{}, fmt.Errorf("surface verifier requires an isolated command runner")
	}
	ownedPatterns := append(append([]string(nil), verifier.contract.allowedChanges...), verifier.contract.qualityTestPatterns...)
	ownedPatterns = append(ownedPatterns, verifier.namedResourceOwnershipPatterns(input.ProjectRoot)...)
	checks := []EndpointResult{verifySurfaceOwnership(input.Changes, ownedPatterns)}
	if len(verifier.contract.requiredChanges) > 0 {
		checks = append(checks, verifyRequiredSurfaceChanges(input.Changes, verifier.contract.requiredChanges))
	}
	if len(verifier.contract.qualityTestPatterns) > 0 {
		checks = append(checks, verifyCandidateTestQuality(input.ProjectRoot, input.Changes, verifier.contract.qualityTestPatterns))
	}
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
		if contract.standard {
			checks = append(checks, runStandardProjectChecks(ctx, verifier.runner, verifier.project(input), contract.standardBuilds)...)
			continue
		}
		if contract.probe != nil {
			checks = append(checks, contract.probe(ctx, verifier.runner, verifier.project(input)))
			continue
		}
		resolved, details := resolveNamedResourceProbe(input.ProjectRoot, contract)
		if details != "" {
			checks = append(checks, EndpointResult{ID: contract.id, Status: EndpointFailed, Details: details})
			continue
		}
		checks = append(checks, runIsolatedCommand(ctx, verifier.runner, verifier.project(input), resolved))
	}
	framework := summarizeSurfaceChecks(verifier.ID(), checks)
	return VerificationResult{
		FrameworkOutcome:    framework,
		WorkflowConformance: EndpointResult{ID: "workflow-owned-by-runner", Status: EndpointIneligible},
		Checks:              checks,
	}, nil
}

// project keeps candidate tests out of verifier execution while allowing a reviewed
// contract to omit only baseline tests whose pre-task API is intentionally replaced.
func (verifier *surfaceVerifier) project(input VerificationInput) VerifierProject {
	return VerifierProject{Root: input.ProjectRoot, BaselineTests: input.BaselineTests, BaselineTestExclusions: verifier.contract.baselineTestExclusions}
}

// namedResourceOwnershipPatterns admits only the Wire-connected application's package and its newly created directory.
func (verifier *surfaceVerifier) namedResourceOwnershipPatterns(root string) []string {
	patterns := make([]string, 0)
	for _, source := range verifier.contract.sources {
		if source.providerConnection == nil {
			continue
		}
		paths, err := matchingSurfacePaths(root, source.paths)
		if err != nil {
			continue
		}
		provider, details := connectedNamedResourceProvider(root, paths, source.providerConnection)
		if details == "" {
			patterns = append(patterns, provider.directory, provider.directory+"/*.go")
		}
	}
	return patterns
}

// verifyRequiredSurfaceChanges proves the candidate connected behavior to framework-owned registration points without prescribing application names.
func verifyRequiredSurfaceChanges(changes []ProjectChange, patterns []string) EndpointResult {
	for _, pattern := range patterns {
		matched := false
		for _, change := range changes {
			if change.After.Kind == "" {
				continue
			}
			candidate := filepath.ToSlash(change.Path)
			if ok, _ := filepath.Match(pattern, candidate); ok {
				matched = true
				break
			}
		}
		if !matched {
			return EndpointResult{ID: "required-registration-change", Status: EndpointFailed, Details: fmt.Sprintf("required Project change %q is absent", pattern)}
		}
	}
	return EndpointResult{ID: "required-registration-change", Status: EndpointPassed}
}

// runStandardProjectChecks proves generated parity and compilation in one isolated phase without sharing state with hidden probes.
func runStandardProjectChecks(ctx context.Context, runner CommandRunner, project VerifierProject, buildCommands [][]string) (checks []EndpointResult) {
	session, err := runner.Open(ctx, project)
	if err != nil {
		return []EndpointResult{{ID: "wire-output-parity", Status: EndpointFailed, Details: fmt.Sprintf("open isolated verifier session: %v", err)}}
	}
	defer func() {
		cleanupContext, cancel := context.WithTimeout(context.Background(), verifierCleanupTimeout)
		defer cancel()
		if closeErr := session.Close(cleanupContext); closeErr != nil && !surfaceChecksFailed(checks) {
			checks = append(checks, EndpointResult{ID: "project-compile", Status: EndpointFailed, Details: fmt.Sprintf("close isolated verifier session: %v", closeErr)})
		}
	}()
	parity := verifyWireOutputParityInSession(ctx, session, buildCommands)
	checks = append(checks, parity)
	if parity.Status != EndpointPassed {
		return checks
	}
	return append(checks, runCheck(ctx, session, "project-compile", []string{"go", "test", "./..."}, ""))
}

// runReconcileScheduleBehaviorProbe derives the selected schedule constructor so
// the behavior oracle follows a coherent application-owned name and package.
func runReconcileScheduleBehaviorProbe(ctx context.Context, runner CommandRunner, project VerifierProject) EndpointResult {
	constructor, directory, packageName, err := scheduleBehaviorTarget(project.Root)
	if err != nil {
		return EndpointResult{ID: "reconcile-schedule-behavior", Status: EndpointFailed, Details: err.Error()}
	}
	body := strings.Replace(reconcileScheduleBehaviorProbe, "package invoices\n", "package "+packageName+"\n", 1)
	body = strings.ReplaceAll(body, "NewReconcileSchedule", constructor)
	return runIsolatedCommand(ctx, runner, project, commandContract{
		id:              "reconcile-schedule-behavior",
		arguments:       []string{"go", "test", "./" + directory, "-run", "^TestAtlasReconcileScheduleBehavior$", "-count=1"},
		supervisorFiles: []supervisorFile{{path: filepath.ToSlash(filepath.Join(directory, "atlas_eval_reconcile_schedule_test.go")), body: body}},
	})
}

// scheduleBehaviorTarget finds the single constructor for a type that exposes the
// schedule protocol, avoiding a generator-specific file or constructor spelling.
func scheduleBehaviorTarget(root string) (string, string, string, error) {
	paths, err := matchingSurfacePaths(root, []string{"internal/*/*.go", "app/*_schedule.go"})
	if err != nil {
		return "", "", "", err
	}
	type target struct{ constructor, directory, packageName string }
	var targets []target
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if parseErr != nil {
			continue
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Recv != nil || !strings.HasPrefix(function.Name.Name, "New") || !strings.Contains(function.Name.Name, "Schedule") {
				continue
			}
			directory, relativeErr := filepath.Rel(root, filepath.Dir(path))
			if relativeErr == nil {
				targets = append(targets, target{function.Name.Name, filepath.ToSlash(directory), file.Name.Name})
			}
		}
	}
	if len(targets) != 1 {
		return "", "", "", fmt.Errorf("expected one application schedule constructor, found %d", len(targets))
	}
	return targets[0].constructor, targets[0].directory, targets[0].packageName, nil
}

// surfaceChecksFailed avoids executing candidate code after sealed static evidence already proves failure.
func surfaceChecksFailed(checks []EndpointResult) bool {
	for _, check := range checks {
		if check.Kind != RequirementQuality && check.Status == EndpointFailed {
			return true
		}
	}
	return false
}

// verifyCandidateTestQuality reports whether the candidate authored focused tests without trusting those tests as outcome evidence.
func verifyCandidateTestQuality(root string, changes []ProjectChange, patterns []string) EndpointResult {
	for _, change := range changes {
		candidatePath := filepath.ToSlash(change.Path)
		if change.After.Kind == "" || !strings.HasSuffix(candidatePath, "_test.go") {
			continue
		}
		for _, pattern := range patterns {
			matched, _ := filepath.Match(pattern, candidatePath)
			if matched {
				file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, filepath.FromSlash(candidatePath)), nil, 0)
				if err != nil {
					return EndpointResult{ID: "test-function-added", Kind: RequirementQuality, Status: EndpointFailed, Details: fmt.Sprintf("parse focused test %q: %v", candidatePath, err)}
				}
				for _, declaration := range file.Decls {
					function, ok := declaration.(*ast.FuncDecl)
					if ok && isGoTestFunction(file, function) {
						return EndpointResult{ID: "test-function-added", Kind: RequirementQuality, Status: EndpointPassed}
					}
				}
				return EndpointResult{ID: "test-function-added", Kind: RequirementQuality, Status: EndpointFailed, Details: fmt.Sprintf("focused test %q does not declare a Go test function", candidatePath)}
			}
		}
	}
	return EndpointResult{
		ID:      "test-function-added",
		Kind:    RequirementQuality,
		Status:  EndpointFailed,
		Details: "candidate did not add or update a focused test in the evaluated surface",
	}
}

// isGoTestFunction recognizes the testing package through its actual import name so helpers cannot impersonate focused tests.
func isGoTestFunction(file *ast.File, function *ast.FuncDecl) bool {
	if function.Recv != nil || !isGoTestName(function.Name.Name) || function.Type.Params == nil || len(function.Type.Params.List) != 1 || len(function.Type.Params.List[0].Names) > 1 {
		return false
	}
	testingName := ""
	for _, imported := range file.Imports {
		if imported.Path.Value != `"testing"` {
			continue
		}
		if imported.Name == nil {
			testingName = "testing"
		} else if imported.Name.Name != "." && imported.Name.Name != "_" {
			testingName = imported.Name.Name
		}
		break
	}
	pointer, ok := function.Type.Params.List[0].Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	selector, ok := pointer.X.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "T" {
		return false
	}
	packageName, ok := selector.X.(*ast.Ident)
	return ok && testingName != "" && packageName.Name == testingName
}

// isGoTestName follows the Go tool's naming boundary so ordinary helpers such as Tester are not reported as runnable tests.
func isGoTestName(name string) bool {
	if !strings.HasPrefix(name, "Test") {
		return false
	}
	suffix := name[len("Test"):]
	if suffix == "" {
		return true
	}
	next, _ := utf8.DecodeRuneInString(suffix)
	return !unicode.IsLower(next)
}

// verifySurfaceTextAbsent protects an owning App or generated boundary from cross-surface registration.
func verifySurfaceTextAbsent(root string, exclusion textExclusion) EndpointResult {
	paths, err := matchingSurfacePaths(root, exclusion.paths)
	if err != nil {
		return EndpointResult{ID: exclusion.id, Status: EndpointFailed, Details: err.Error()}
	}
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return EndpointResult{ID: exclusion.id, Status: EndpointFailed, Details: err.Error()}
		}
		contains, err := surfaceSourceContains(path, body, exclusion.text)
		if err != nil {
			return EndpointResult{ID: exclusion.id, Status: EndpointFailed, Details: err.Error()}
		}
		if contains {
			return EndpointResult{ID: exclusion.id, Status: EndpointFailed, Details: fmt.Sprintf("protected path %q contains %q", filepath.ToSlash(path), exclusion.text)}
		}
	}
	return EndpointResult{ID: exclusion.id, Status: EndpointPassed}
}

// surfaceSourceContains ignores Go comments while preserving checks against syntax and string literals.
func surfaceSourceContains(path string, body []byte, text string) (bool, error) {
	if filepath.Ext(path) != ".go" {
		return strings.Contains(string(body), text), nil
	}
	file, err := parser.ParseFile(token.NewFileSet(), path, body, 0)
	if err != nil {
		return false, fmt.Errorf("parse protected path %q: %w", filepath.ToSlash(path), err)
	}
	found := false
	ast.Inspect(file, func(node ast.Node) bool {
		if found || node == nil {
			return false
		}
		switch typed := node.(type) {
		case *ast.Ident:
			found = strings.Contains(typed.Name, text)
		case *ast.BasicLit:
			found = strings.Contains(typed.Value, text)
		}
		return !found
	})
	return found, nil
}

// verifySurfaceOwnership limits candidate changes while allowing regenerated Wire output as derived evidence.
func verifySurfaceOwnership(changes []ProjectChange, patterns []string) EndpointResult {
	var unrelated []string
	for _, change := range changes {
		path := filepath.ToSlash(change.Path)
		if derivedSurfaceProjectChange(change) {
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

// derivedSurfaceProjectChange accepts empty runtime storage directories without exempting source files in a package named storage.
func derivedSurfaceProjectChange(change ProjectChange) bool {
	path := filepath.ToSlash(change.Path)
	if derivedSurfaceChange(path) {
		return true
	}
	kind := change.After.Kind
	if kind == "" {
		kind = change.Before.Kind
	}
	return kind == "directory" && (strings.Contains(path, "/storage/") || strings.HasSuffix(path, "/storage"))
}

// derivedSurfaceChange identifies framework and Go tool outputs that are verified through isolated commands rather than authored ownership.
func derivedSurfaceChange(path string) bool {
	if path == "go.sum" || filepath.Base(path) == "wire_gen.go" {
		return true
	}
	if path == "_data" || strings.Contains(path, "/_data/") || strings.HasSuffix(path, "/_data") {
		return true
	}
	return path == "bin" || path == "build" || path == "storage" ||
		strings.HasPrefix(path, "bin/") || strings.HasPrefix(path, "build/") || strings.HasPrefix(path, "storage/")
}

// verifySurfaceSource finds syntax-bearing facts without counting comments as implementation evidence.
func verifySurfaceSource(root string, contract sourceContract) EndpointResult {
	paths, err := matchingSurfacePaths(root, contract.paths)
	if err != nil {
		return EndpointResult{ID: contract.id, Status: EndpointFailed, Details: err.Error()}
	}
	facts := newSourceFacts()
	declarations := map[string]*sourceFacts{}
	declarationNodes := map[string]ast.Node{}
	assignments := map[string]*sourceFacts{}
	var text strings.Builder
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
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
		collectSourceFacts(file, &facts, declarations, declarationNodes, assignments)
	}
	if details := verifySourceFacts(facts, contract.identifiers, contract.identifierChoices, contract.selectorCalls, contract.forbiddenCalls, contract.stringLiterals, contract.forbiddenLiterals); details != "" {
		return EndpointResult{ID: contract.id, Status: EndpointFailed, Details: details}
	}
	for _, declaration := range contract.declarations {
		names := append([]string(nil), declaration.nameChoices...)
		if declaration.name != "" {
			names = append([]string{declaration.name}, names...)
		}
		key := ""
		var scope *sourceFacts
		for _, name := range names {
			candidate := name
			if declaration.receiver != "" {
				candidate = declaration.receiver + "." + name
			}
			if declarations[candidate] != nil {
				key = candidate
				scope = declarations[candidate]
				break
			}
		}
		if scope == nil && declaration.anyName {
			for candidate, candidateScope := range declarations {
				if declaration.receiver != "" && !strings.HasPrefix(candidate, declaration.receiver+".") {
					continue
				}
				if declarationFactsMismatch(candidateScope, declaration) == "" {
					key = candidate
					scope = candidateScope
					break
				}
			}
		}
		if scope == nil {
			return EndpointResult{ID: contract.id, Status: EndpointFailed, Details: fmt.Sprintf("required declaration %q is absent", names)}
		}
		if details := declarationFactsMismatch(scope, declaration); details != "" {
			return EndpointResult{ID: contract.id, Status: EndpointFailed, Details: fmt.Sprintf("declaration %q: %s", key, details)}
		}
		for _, identifier := range declaration.forbiddenIdentifiers {
			if scope.identifiers[identifier] {
				return EndpointResult{ID: contract.id, Status: EndpointFailed, Details: fmt.Sprintf("declaration %q contains forbidden identifier %q", key, identifier)}
			}
		}
		for _, nested := range declaration.nestedCalls {
			if !scope.nestedCalls[nested.outer+">"+nested.inner] {
				return EndpointResult{ID: contract.id, Status: EndpointFailed, Details: fmt.Sprintf("declaration %q does not call %q within %q", key, nested.inner, nested.outer)}
			}
		}
		for _, flow := range declaration.argumentFlows {
			if !declarationArgumentFlowsFromLiteral(declarationNodes[key], flow) {
				return EndpointResult{ID: contract.id, Status: EndpointFailed, Details: fmt.Sprintf("declaration %q does not pass a value resolved from %q to %q", key, flow.literal, flow.call)}
			}
		}
	}
	for _, assignment := range contract.assignments {
		scope := assignments[assignment.name]
		if scope == nil {
			return EndpointResult{ID: contract.id, Status: EndpointFailed, Details: fmt.Sprintf("required assignment %q is absent", assignment.name)}
		}
		if details := verifySourceFacts(*scope, assignment.identifiers, nil, assignment.selectorCalls, nil, nil, nil); details != "" {
			return EndpointResult{ID: contract.id, Status: EndpointFailed, Details: fmt.Sprintf("assignment %q: %s", assignment.name, details)}
		}
		for _, identifier := range assignment.forbiddenIdentifiers {
			if scope.identifiers[identifier] {
				return EndpointResult{ID: contract.id, Status: EndpointFailed, Details: fmt.Sprintf("assignment %q contains forbidden identifier %q", assignment.name, identifier)}
			}
		}
	}
	for _, required := range contract.text {
		if !strings.Contains(text.String(), required) {
			return EndpointResult{ID: contract.id, Status: EndpointFailed, Details: fmt.Sprintf("required configuration %q is absent", required)}
		}
	}
	normalized := normalizeSurfaceText(text.String())
	for _, required := range contract.normalizedText {
		if !strings.Contains(normalized, normalizeSurfaceText(required)) {
			return EndpointResult{ID: contract.id, Status: EndpointFailed, Details: fmt.Sprintf("required configuration %q is absent", required)}
		}
	}
	if contract.appConfiguration != nil {
		if details := verifyAppConfiguration(text.String(), *contract.appConfiguration); details != "" {
			return EndpointResult{ID: contract.id, Status: EndpointFailed, Details: details}
		}
	}
	if contract.commentOnly && !sqlCommentsOnly(text.String()) {
		return EndpointResult{ID: contract.id, Status: EndpointFailed, Details: "migration must not contain executable SQL"}
	}
	for _, change := range contract.sqlColumnChanges {
		if !sqlChangesColumn(text.String(), change) {
			verb := "remove"
			if change.add {
				verb = "add"
			}
			return EndpointResult{ID: contract.id, Status: EndpointFailed, Details: fmt.Sprintf("migration does not %s column %q on table %q", verb, change.column, change.table)}
		}
	}
	if contract.providerConnection != nil {
		if details := verifyProviderConnection(root, paths, contract.providerConnection); details != "" {
			return EndpointResult{ID: contract.id, Status: EndpointFailed, Details: details}
		}
	}
	if contract.scheduleRegistration {
		if details := verifyScheduleRegistration(root, paths); details != "" {
			return EndpointResult{ID: contract.id, Status: EndpointFailed, Details: details}
		}
	}
	for _, routeGroup := range contract.routeGroups {
		if !sourceHasRouteGroup(facts.routeGroupCalls, routeGroup) {
			return EndpointResult{ID: contract.id, Status: EndpointFailed, Details: fmt.Sprintf("route group does not join %q with middleware %q", routeGroup.routesIdentifier, routeGroup.middlewareSelector)}
		}
	}
	return EndpointResult{ID: contract.id, Status: EndpointPassed}
}

// verifyScheduleRegistration requires an application registration to reference the constructor for a concrete schedule shape.
func verifyScheduleRegistration(root string, paths []string) string {
	candidates := scheduleCandidates(paths)
	if len(candidates) == 0 {
		return "no schedule with Interval and Handle methods is present"
	}
	for _, path := range paths {
		if !isApplicationSource(root, path) {
			continue
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err.Error()
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, body, 0)
		if err != nil {
			return fmt.Sprintf("parse %s: %v", filepath.ToSlash(path), err)
		}
		if applicationRegistersSchedule(file, root, path, candidates) {
			return ""
		}
	}
	return "no application registration references a concrete schedule constructor"
}

// scheduleCandidate identifies the constructor for a type that implements the supported recurring schedule shape.
type scheduleCandidate struct {
	name        string
	constructor string
	directory   string
}

// scheduleTypeKey keeps platform-specific path syntax separate from the Go type name.
type scheduleTypeKey struct {
	directory string
	name      string
}

// scheduleCandidates discovers schedule constructors from their methods instead of their file or package names.
func scheduleCandidates(paths []string) []scheduleCandidate {
	types := map[scheduleTypeKey]map[string]bool{}
	constructors := map[scheduleTypeKey]bool{}
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") || filepath.Ext(path) != ".go" {
			continue
		}
		body, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, body, 0)
		if err != nil {
			continue
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if receiver := functionReceiverName(function); receiver != "" && (function.Name.Name == "Interval" || function.Name.Name == "Handle") {
				key := scheduleTypeKey{directory: filepath.Dir(path), name: receiver}
				if types[key] == nil {
					types[key] = map[string]bool{}
				}
				types[key][function.Name.Name] = true
			}
			if function.Recv == nil && strings.HasPrefix(function.Name.Name, "New") {
				if typeName := functionResultType(function); typeName != "" && function.Name.Name == "New"+typeName {
					constructors[scheduleTypeKey{directory: filepath.Dir(path), name: typeName}] = true
				}
			}
		}
	}
	candidates := make([]scheduleCandidate, 0)
	for key, methods := range types {
		if !methods["Interval"] || !methods["Handle"] {
			continue
		}
		if !constructors[key] {
			continue
		}
		candidates = append(candidates, scheduleCandidate{name: key.name, constructor: "New" + key.name, directory: key.directory})
	}
	sort.Slice(candidates, func(left, right int) bool {
		if candidates[left].directory != candidates[right].directory {
			return candidates[left].directory < candidates[right].directory
		}
		return candidates[left].name < candidates[right].name
	})
	return candidates
}

// functionResultType returns the named type produced by a constructor, accepting pointer and value constructors.
func functionResultType(function *ast.FuncDecl) string {
	if function == nil || function.Type.Results == nil || len(function.Type.Results.List) != 1 {
		return ""
	}
	expression := function.Type.Results.List[0].Type
	if pointer, ok := expression.(*ast.StarExpr); ok {
		expression = pointer.X
	}
	identifier, _ := expression.(*ast.Ident)
	if identifier == nil {
		return ""
	}
	return identifier.Name
}

// isApplicationSource limits registration evidence to application wiring rather than a detached domain helper.
func isApplicationSource(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && strings.Split(filepath.ToSlash(relative), "/")[0] == "app"
}

// applicationRegistersSchedule recognizes generated Wire sets and direct AppSchedules construction without relying on a particular filename.
func applicationRegistersSchedule(file *ast.File, root, path string, candidates []scheduleCandidate) bool {
	imports := wireImportPaths(file)
	found := false
	ast.Inspect(file, func(current ast.Node) bool {
		call, ok := current.(*ast.CallExpr)
		if !ok || (!isWireNewSet(call) && callName(call) != "NewAppSchedules") {
			return true
		}
		for _, argument := range call.Args {
			if scheduleConstructorReference(argument, imports, root, path, candidates) {
				found = true
				return false
			}
		}
		return !found
	})
	return found
}

// scheduleConstructorReference matches a constructor reference to the package that declares its schedule type.
func scheduleConstructorReference(expression ast.Expr, imports map[string]string, root, path string, candidates []scheduleCandidate) bool {
	if call, ok := expression.(*ast.CallExpr); ok {
		expression = call.Fun
	}
	if identifier, ok := expression.(*ast.Ident); ok {
		for _, candidate := range candidates {
			if identifier.Name == candidate.constructor && candidate.directory == filepath.Dir(path) {
				return true
			}
		}
		return false
	}
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	packageName, ok := selector.X.(*ast.Ident)
	if !ok {
		return false
	}
	for _, candidate := range candidates {
		if selector.Sel.Name != candidate.constructor {
			continue
		}
		relative, err := filepath.Rel(root, candidate.directory)
		if err == nil && strings.HasSuffix(imports[packageName.Name], "/"+filepath.ToSlash(relative)) {
			return true
		}
	}
	return false
}

// sourceHasRouteGroup reports whether one NewRouteGroup call applies the required middleware to the required route slice.
func sourceHasRouteGroup(calls []*ast.CallExpr, contract routeGroupContract) bool {
	for _, call := range calls {
		if callName(call) != "NewRouteGroup" {
			continue
		}
		foundRoutes := false
		foundMiddleware := false
		for _, argument := range call.Args {
			if identifier, ok := argument.(*ast.Ident); ok && identifier.Name == contract.routesIdentifier {
				foundRoutes = true
			}
			if expressionCallName(argument) == contract.middlewareSelector {
				foundMiddleware = true
			}
		}
		if foundRoutes && foundMiddleware {
			return true
		}
	}
	return false
}

// verifyProviderConnection finds an application-owned provider that resolves the named resource and confirms App Wire registers that same provider.
func verifyProviderConnection(root string, sourcePaths []string, contract *providerConnectionContract) string {
	_, details := connectedNamedResourceProvider(root, sourcePaths, contract)
	return details
}

// connectedNamedResourceProvider returns the one accessor provider registered by App Wire.
func connectedNamedResourceProvider(root string, sourcePaths []string, contract *providerConnectionContract) (namedResourceProvider, string) {
	providers := namedResourceProviders(root, sourcePaths, contract)
	if len(providers) == 0 {
		return namedResourceProvider{}, fmt.Sprintf("no provider resolves named accessor %q", contract.accessor)
	}
	wirePaths, err := matchingSurfacePaths(root, contract.wirePaths)
	if err != nil {
		return namedResourceProvider{}, err.Error()
	}
	connected := make([]namedResourceProvider, 0, 1)
	seen := map[string]bool{}
	for _, provider := range providers {
		for _, path := range wirePaths {
			body, readErr := os.ReadFile(path)
			if readErr != nil {
				return namedResourceProvider{}, readErr.Error()
			}
			file, parseErr := parser.ParseFile(token.NewFileSet(), path, body, 0)
			if parseErr != nil {
				return namedResourceProvider{}, fmt.Sprintf("parse %s: %v", filepath.ToSlash(path), parseErr)
			}
			if wireSetReferencesProvider(file, []namedResourceProvider{provider}) {
				key := provider.directory + ":" + provider.name
				if !seen[key] {
					connected = append(connected, provider)
					seen[key] = true
				}
			}
		}
	}
	if len(connected) != 1 {
		return namedResourceProvider{}, fmt.Sprintf("no unique provider resolving %q is registered in App wire.NewSet", contract.accessor)
	}
	return connected[0], ""
}

// namedResourceProviders returns function declarations that directly resolve the selected named resource.
func namedResourceProviders(root string, paths []string, contract *providerConnectionContract) []namedResourceProvider {
	providers := make([]namedResourceProvider, 0)
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") || filepath.Ext(path) != ".go" {
			continue
		}
		body, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, body, 0)
		if err != nil {
			continue
		}
		imports := wireImportPaths(file)
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Recv != nil || !functionCallsAccessor(function, contract.accessor) || !functionReceivesManager(function, imports, contract.managerImportSuffix) {
				continue
			}
			directory, relativeErr := filepath.Rel(root, filepath.Dir(path))
			if relativeErr != nil {
				continue
			}
			providers = append(providers, namedResourceProvider{name: function.Name.Name, directory: filepath.ToSlash(directory), packageName: file.Name.Name})
		}
	}
	return providers
}

// functionReceivesManager binds the accessor call to the generated manager dependency rather than an unrelated method with the same name.
func functionReceivesManager(function *ast.FuncDecl, imports map[string]string, importSuffix string) bool {
	if function == nil || function.Type.Params == nil || importSuffix == "" {
		return false
	}
	for _, field := range function.Type.Params.List {
		pointer, ok := field.Type.(*ast.StarExpr)
		if !ok {
			continue
		}
		selector, ok := pointer.X.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "Manager" {
			continue
		}
		packageName, ok := selector.X.(*ast.Ident)
		if ok && strings.HasSuffix(imports[packageName.Name], importSuffix) {
			return true
		}
	}
	return false
}

// namedResourceProvider identifies a provider by its declaration name and application package directory.
type namedResourceProvider struct {
	name        string
	directory   string
	packageName string
}

// resolveNamedResourceProbe installs the probe beside the provider that App Wire actually uses.
func resolveNamedResourceProbe(root string, contract commandContract) (commandContract, string) {
	if contract.namedResourceProbe == nil {
		return contract, ""
	}
	paths, err := matchingSurfacePaths(root, []string{"internal/*/*.go", "app/*.go"})
	if err != nil {
		return commandContract{}, err.Error()
	}
	provider, details := connectedNamedResourceProvider(root, paths, contract.namedResourceProbe)
	if details != "" {
		return commandContract{}, details
	}
	for index := range contract.supervisorFiles {
		file := &contract.supervisorFiles[index]
		file.path = filepath.ToSlash(filepath.Join(provider.directory, filepath.Base(file.path)))
		file.body = strings.Replace(file.body, "package invoices\n", "package "+provider.packageName+"\n", 1)
	}
	for index, argument := range contract.arguments {
		if strings.HasPrefix(argument, "./internal/") {
			contract.arguments[index] = "./" + provider.directory
		}
	}
	return contract, ""
}

// functionCallsAccessor identifies providers by their resource use rather than an application-specific function name.
func functionCallsAccessor(function *ast.FuncDecl, accessor string) bool {
	found := false
	ast.Inspect(function, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if ok && callName(call) == accessor {
			found = true
			return false
		}
		return !found
	})
	return found
}

// wireSetReferencesProvider proves that a provider discovered from resource use is an argument to an App services wire.NewSet call.
func wireSetReferencesProvider(file *ast.File, providers []namedResourceProvider) bool {
	imports := wireImportPaths(file)
	found := false
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || !isWireNewSet(call) {
			return true
		}
		for _, argument := range call.Args {
			if wireArgumentReferencesProvider(argument, imports, providers) {
				found = true
				return false
			}
		}
		return !found
	})
	return found
}

// wireImportPaths maps the local import spelling to its source path so identically named providers in sibling packages cannot satisfy the connection.
func wireImportPaths(file *ast.File) map[string]string {
	imports := map[string]string{}
	for _, imported := range file.Imports {
		importPath := strings.Trim(imported.Path.Value, "\"")
		name := ""
		if imported.Name == nil {
			name = filepath.Base(importPath)
		} else {
			name = imported.Name.Name
		}
		imports[name] = importPath
	}
	return imports
}

// wireArgumentReferencesProvider compares both the provider name and its imported application package.
func wireArgumentReferencesProvider(argument ast.Expr, imports map[string]string, providers []namedResourceProvider) bool {
	selector, ok := argument.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	packageName, ok := selector.X.(*ast.Ident)
	if !ok {
		return false
	}
	importPath := imports[packageName.Name]
	for _, provider := range providers {
		if provider.name == selector.Sel.Name && strings.HasSuffix(importPath, "/"+provider.directory) {
			return true
		}
	}
	return false
}

// isWireNewSet recognizes the framework's provider-set constructor without accepting unrelated NewSet helpers.
func isWireNewSet(call *ast.CallExpr) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "NewSet" {
		return false
	}
	packageName, ok := selector.X.(*ast.Ident)
	return ok && packageName.Name == "wire"
}

// declarationFactsMismatch keeps named and structurally selected declarations on the same evidence contract.
func declarationFactsMismatch(scope *sourceFacts, declaration declarationContract) string {
	if scope == nil {
		return "declaration is absent"
	}
	if details := verifySourceFacts(*scope, declaration.identifiers, nil, declaration.selectorCalls, declaration.forbiddenCalls, declaration.stringLiterals, declaration.forbiddenLiterals); details != "" {
		return details
	}
	for _, identifier := range declaration.forbiddenIdentifiers {
		if scope.identifiers[identifier] {
			return fmt.Sprintf("contains forbidden identifier %q", identifier)
		}
	}
	for _, nested := range declaration.nestedCalls {
		if !scope.nestedCalls[nested.outer+">"+nested.inner] {
			return fmt.Sprintf("does not call %q within %q", nested.inner, nested.outer)
		}
	}
	return ""
}

// normalizeSurfaceText tolerates presentation spacing around configuration separators without merging distinct tokens.
func normalizeSurfaceText(source string) string {
	normalized := strings.Join(strings.Fields(source), " ")
	for _, separator := range []string{"->", ":"} {
		normalized = strings.ReplaceAll(normalized, " "+separator, separator)
		normalized = strings.ReplaceAll(normalized, separator+" ", separator)
	}
	return normalized
}

// verifyAppConfiguration distinguishes persisted App capabilities from the separate dev.apps watcher graph.
func verifyAppConfiguration(source string, contract appConfigurationContract) string {
	var configuration map[string]any
	if err := yaml.Unmarshal([]byte(source), &configuration); err != nil {
		return fmt.Sprintf("invalid Project configuration: %v", err)
	}
	apps, ok := configuration["apps"].(map[string]any)
	if !ok {
		return fmt.Sprintf("required App configuration %q is absent", contract.name)
	}
	app, ok := apps[contract.name].(map[string]any)
	if !ok {
		return fmt.Sprintf("required App configuration %q is absent", contract.name)
	}
	components, ok := app["components"]
	if !ok {
		return fmt.Sprintf("App configuration %q does not declare components", contract.name)
	}
	for _, required := range contract.requiredComponents {
		if !appConfigurationHasComponent(components, required) {
			return fmt.Sprintf("App configuration %q does not enable component %q", contract.name, required)
		}
	}
	return ""
}

// appConfigurationHasComponent accepts GoForj's sequence and mapping component encodings.
func appConfigurationHasComponent(components any, required string) bool {
	switch values := components.(type) {
	case []any:
		for _, value := range values {
			if component, ok := value.(string); ok && component == required {
				return true
			}
		}
	case map[string]any:
		enabled, ok := values[required].(bool)
		return ok && enabled
	}
	return false
}

// sqlCommentsOnly accepts SQL comment syntax without treating a generator's particular comment wording as an outcome requirement.
func sqlCommentsOnly(source string) bool {
	inBlockComment := false
	for index := 0; index < len(source); {
		if inBlockComment {
			end := strings.Index(source[index:], "*/")
			if end < 0 {
				return false
			}
			index += end + 2
			inBlockComment = false
			continue
		}
		if source[index] == '/' && index+1 < len(source) && source[index+1] == '*' {
			inBlockComment = true
			index += 2
			continue
		}
		if source[index] == '-' && index+1 < len(source) && source[index+1] == '-' {
			newline := strings.IndexByte(source[index:], '\n')
			if newline < 0 {
				return true
			}
			index += newline + 1
			continue
		}
		character, size := utf8.DecodeRuneInString(source[index:])
		if !unicode.IsSpace(character) {
			return false
		}
		index += size
	}
	return !inBlockComment
}

// sqlChangesColumn recognizes the portable ALTER TABLE forms that add or remove one named column.
func sqlChangesColumn(source string, change sqlColumnChangeContract) bool {
	tokens, ok := sqlTokens(source)
	if !ok {
		return false
	}
	for index := 0; index+3 < len(tokens); index++ {
		if tokens[index] != "alter" || tokens[index+1] != "table" {
			continue
		}
		index += 2
		for index < len(tokens) && (tokens[index] == "if" || tokens[index] == "exists" || tokens[index] == "only") {
			index++
		}
		if index >= len(tokens) || tokens[index] != strings.ToLower(change.table) {
			continue
		}
		index++
		if index >= len(tokens) {
			continue
		}
		if change.add && tokens[index] == "add" {
			index++
			if index < len(tokens) && tokens[index] == "column" {
				index++
			}
			for index < len(tokens) && (tokens[index] == "if" || tokens[index] == "not" || tokens[index] == "exists") {
				index++
			}
		} else if !change.add && tokens[index] == "drop" {
			index++
			if index < len(tokens) && tokens[index] == "column" {
				index++
			}
			for index < len(tokens) && (tokens[index] == "if" || tokens[index] == "exists") {
				index++
			}
		} else {
			continue
		}
		if index < len(tokens) && tokens[index] == strings.ToLower(change.column) {
			return true
		}
	}
	return false
}

// sqlTokens extracts SQL keywords and identifiers while ignoring comments and string literals.
func sqlTokens(source string) ([]string, bool) {
	tokens := make([]string, 0)
	for index := 0; index < len(source); {
		if unicode.IsSpace(rune(source[index])) {
			index++
			continue
		}
		if index+1 < len(source) && source[index:index+2] == "--" {
			newline := strings.IndexByte(source[index:], '\n')
			if newline < 0 {
				break
			}
			index += newline + 1
			continue
		}
		if index+1 < len(source) && source[index:index+2] == "/*" {
			end := strings.Index(source[index+2:], "*/")
			if end < 0 {
				return nil, false
			}
			index += end + 4
			continue
		}
		if source[index] == '\'' {
			end, ok := sqlQuotedTokenEnd(source, index, '\'')
			if !ok {
				return nil, false
			}
			index = end
			continue
		}
		if source[index] == '"' || source[index] == '`' || source[index] == '[' {
			quote := source[index]
			endQuote := quote
			if quote == '[' {
				endQuote = ']'
			}
			end, ok := sqlQuotedTokenEnd(source, index, endQuote)
			if !ok {
				return nil, false
			}
			tokens = append(tokens, strings.ToLower(source[index+1:end-1]))
			index = end
			continue
		}
		if isSQLTokenCharacter(source[index]) {
			end := index + 1
			for end < len(source) && isSQLTokenCharacter(source[end]) {
				end++
			}
			tokens = append(tokens, strings.ToLower(source[index:end]))
			index = end
			continue
		}
		index++
	}
	return tokens, true
}

// sqlQuotedTokenEnd returns the byte immediately after a SQL quoted token, accepting doubled quote escapes.
func sqlQuotedTokenEnd(source string, start int, quote byte) (int, bool) {
	for index := start + 1; index < len(source); index++ {
		if source[index] != quote {
			continue
		}
		if index+1 < len(source) && source[index+1] == quote {
			index++
			continue
		}
		return index + 1, true
	}
	return 0, false
}

// isSQLTokenCharacter keeps the SQL lexer deliberately narrow because this verifier only needs keywords and identifiers.
func isSQLTokenCharacter(value byte) bool {
	return value == '_' || value == '$' || value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9'
}

// sourceFacts retains the syntax classes used by promoted surface contracts.
type sourceFacts struct {
	identifiers     map[string]bool
	selectorCalls   map[string]bool
	stringLiterals  map[string]bool
	nestedCalls     map[string]bool
	routeGroupCalls []*ast.CallExpr
}

// newSourceFacts creates one isolated syntax evidence set.
func newSourceFacts() sourceFacts {
	return sourceFacts{
		identifiers:    map[string]bool{},
		selectorCalls:  map[string]bool{},
		stringLiterals: map[string]bool{},
		nestedCalls:    map[string]bool{},
	}
}

// verifySourceFacts applies shared syntax requirements without weakening declaration-scoped checks into file-wide evidence.
func verifySourceFacts(facts sourceFacts, identifiers []string, identifierChoices [][]string, selectorCalls, forbiddenCalls, stringLiterals, forbiddenLiterals []string) string {
	for _, identifier := range identifiers {
		if !facts.identifiers[identifier] {
			return fmt.Sprintf("required identifier %q is absent", identifier)
		}
	}
	for _, choices := range identifierChoices {
		matched := false
		for _, identifier := range choices {
			if facts.identifiers[identifier] {
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Sprintf("one of the required identifiers %q is absent", choices)
		}
	}
	for _, selector := range selectorCalls {
		if !facts.selectorCalls[selector] {
			return fmt.Sprintf("required call %q is absent", selector)
		}
	}
	for _, selector := range forbiddenCalls {
		if facts.selectorCalls[selector] {
			return fmt.Sprintf("forbidden call %q is present", selector)
		}
	}
	for _, literal := range stringLiterals {
		if !facts.stringLiterals[literal] {
			return fmt.Sprintf("required string literal %q is absent", literal)
		}
	}
	for _, literal := range forbiddenLiterals {
		if facts.stringLiterals[literal] {
			return fmt.Sprintf("forbidden string literal %q is present", literal)
		}
	}
	return ""
}

// collectSourceFacts records package-wide and named-declaration evidence while excluding comments.
func collectSourceFacts(file *ast.File, facts *sourceFacts, declarations map[string]*sourceFacts, declarationNodes map[string]ast.Node, assignments map[string]*sourceFacts) {
	collectNodeFacts(file, facts)
	for _, declaration := range file.Decls {
		switch value := declaration.(type) {
		case *ast.FuncDecl:
			collectDeclarationFacts(value.Name.Name, value, declarations, declarationNodes)
			if receiver := functionReceiverName(value); receiver != "" {
				collectDeclarationFacts(receiver+"."+value.Name.Name, value, declarations, declarationNodes)
			}
		case *ast.GenDecl:
			for _, spec := range value.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					collectDeclarationFacts(typeSpec.Name.Name, typeSpec, declarations, declarationNodes)
				}
			}
		}
	}
	ast.Inspect(file, func(node ast.Node) bool {
		assignment, ok := node.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for index, expression := range assignment.Lhs {
			identifier, ok := expression.(*ast.Ident)
			if !ok || index >= len(assignment.Rhs) {
				continue
			}
			facts := assignments[identifier.Name]
			if facts == nil {
				value := newSourceFacts()
				facts = &value
				assignments[identifier.Name] = facts
			}
			collectNodeFacts(assignment.Rhs[index], facts)
		}
		return true
	})
}

// functionReceiverName identifies a method owner without coupling contracts to pointer or value syntax.
func functionReceiverName(declaration *ast.FuncDecl) string {
	if declaration == nil || declaration.Recv == nil || len(declaration.Recv.List) != 1 {
		return ""
	}
	expression := declaration.Recv.List[0].Type
	if pointer, ok := expression.(*ast.StarExpr); ok {
		expression = pointer.X
	}
	identifier, _ := expression.(*ast.Ident)
	if identifier == nil {
		return ""
	}
	return identifier.Name
}

// collectDeclarationFacts merges declarations with the same name so reviewed method families remain supported.
func collectDeclarationFacts(name string, node ast.Node, declarations map[string]*sourceFacts, declarationNodes map[string]ast.Node) {
	facts := declarations[name]
	if facts == nil {
		value := newSourceFacts()
		facts = &value
		declarations[name] = facts
	}
	declarationNodes[name] = node
	collectNodeFacts(node, facts)
}

// declarationArgumentFlowsFromLiteral follows local values from a literal-bearing accessor call into a constructor call.
func declarationArgumentFlowsFromLiteral(node ast.Node, contract callArgumentFlowContract) bool {
	function, ok := node.(*ast.FuncDecl)
	if !ok || function.Body == nil {
		return false
	}
	values := map[string]ast.Expr{}
	ast.Inspect(function.Body, func(node ast.Node) bool {
		switch statement := node.(type) {
		case *ast.AssignStmt:
			for index, left := range statement.Lhs {
				name, ok := left.(*ast.Ident)
				if ok && index < len(statement.Rhs) {
					values[name.Name] = statement.Rhs[index]
				}
			}
		case *ast.ValueSpec:
			for index, name := range statement.Names {
				if index < len(statement.Values) {
					values[name.Name] = statement.Values[index]
				}
			}
		}
		return true
	})
	resolved := map[string]bool{}
	for changed := true; changed; {
		changed = false
		for name, value := range values {
			if resolved[name] || expressionFlowsFromLiteral(value, contract.literal, resolved) {
				if !resolved[name] {
					resolved[name] = true
					changed = true
				}
			}
		}
	}
	flows := false
	ast.Inspect(function.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || callName(call) != contract.call {
			return true
		}
		for _, argument := range call.Args {
			if expressionFlowsFromLiteral(argument, contract.literal, resolved) {
				flows = true
				return false
			}
		}
		return true
	})
	return flows
}

// expressionFlowsFromLiteral recognizes an accessor call carrying the literal or a local value derived from one.
func expressionFlowsFromLiteral(expression ast.Expr, literal string, resolved map[string]bool) bool {
	found := false
	ast.Inspect(expression, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.Ident:
			if resolved[value.Name] {
				found = true
			}
		case *ast.CallExpr:
			if callName(value) != "" && callContainsStringLiteral(value, literal) {
				found = true
			}
		}
		return !found
	})
	return found
}

// callContainsStringLiteral keeps provider checks independent of the environment or configuration accessor spelling.
func callContainsStringLiteral(call *ast.CallExpr, literal string) bool {
	found := false
	for _, argument := range call.Args {
		ast.Inspect(argument, func(node ast.Node) bool {
			value, ok := node.(*ast.BasicLit)
			if ok && value.Kind == token.STRING && strings.Trim(value.Value, "`\"") == literal {
				found = true
			}
			return !found
		})
		if found {
			return true
		}
	}
	return false
}

// collectNodeFacts records identifiers, calls, literals, and call containment for one syntax boundary.
func collectNodeFacts(node ast.Node, facts *sourceFacts) {
	ast.Inspect(node, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.Ident:
			facts.identifiers[value.Name] = true
		case *ast.CallExpr:
			if callName(value) == "NewRouteGroup" {
				facts.routeGroupCalls = append(facts.routeGroupCalls, value)
			}
			outer := callName(value)
			if outer != "" {
				facts.selectorCalls[outer] = true
				for _, argument := range value.Args {
					ast.Inspect(argument, func(inner ast.Node) bool {
						call, ok := inner.(*ast.CallExpr)
						if ok && call != value {
							if innerName := callName(call); innerName != "" {
								facts.nestedCalls[outer+">"+innerName] = true
							}
						}
						return true
					})
				}
			}
		case *ast.BasicLit:
			if value.Kind == token.STRING {
				facts.stringLiterals[strings.Trim(value.Value, "`\"")] = true
			}
		}
		return true
	})
}

// callName returns the stable terminal name used by reviewed contracts across package and receiver choices.
func callName(call *ast.CallExpr) string {
	return expressionCallName(call.Fun)
}

// expressionCallName unwraps generic instantiation so calls such as cache.Get[User] retain the same semantic name as ordinary calls.
func expressionCallName(expression ast.Expr) string {
	switch value := expression.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.SelectorExpr:
		return value.Sel.Name
	case *ast.IndexExpr:
		return expressionCallName(value.X)
	case *ast.IndexListExpr:
		return expressionCallName(value.X)
	default:
		return ""
	}
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
func runIsolatedCommand(ctx context.Context, runner CommandRunner, project VerifierProject, contract commandContract) (result EndpointResult) {
	session, err := runner.Open(ctx, project)
	if err != nil {
		return EndpointResult{ID: contract.id, Status: EndpointFailed, Details: fmt.Sprintf("open isolated verifier session: %v", err)}
	}
	defer func() {
		cleanupContext, cancel := context.WithTimeout(context.Background(), verifierCleanupTimeout)
		defer cancel()
		if closeErr := session.Close(cleanupContext); closeErr != nil && result.Status == EndpointPassed {
			result = EndpointResult{ID: contract.id, Status: EndpointFailed, Details: fmt.Sprintf("close isolated verifier session: %v", closeErr)}
		}
	}()
	for _, file := range contract.supervisorFiles {
		if err := session.WriteFile(file.path, []byte(file.body)); err != nil {
			return EndpointResult{ID: contract.id, Status: EndpointFailed, Details: fmt.Sprintf("install supervisor probe: %v", err)}
		}
	}
	arguments := append([]string(nil), contract.arguments...)
	contains := append([]string(nil), contract.contains...)
	if len(contract.supervisorFiles) > 0 {
		var marker string
		arguments, marker, err = installSupervisorCompletionMarker(session, contract.supervisorFiles[0], arguments)
		if err != nil {
			return EndpointResult{ID: contract.id, Status: EndpointFailed, Details: fmt.Sprintf("install supervisor completion marker: %v", err)}
		}
		contains = append(contains, marker)
	}
	return runCheck(ctx, session, contract.id, arguments, contains...)
}

// installSupervisorCompletionMarker rejects bare successful exits; adversarial anti-forgery remains the authoritative backend's responsibility.
func installSupervisorCompletionMarker(session CommandSession, source supervisorFile, arguments []string) ([]string, string, error) {
	file, err := parser.ParseFile(token.NewFileSet(), source.path, source.body, parser.PackageClauseOnly)
	if err != nil {
		return nil, "", fmt.Errorf("parse supervisor probe package: %w", err)
	}
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return nil, "", fmt.Errorf("generate completion nonce: %w", err)
	}
	nonce := hex.EncodeToString(random)
	marker := "ATLAS_SUPERVISOR_COMPLETION_" + nonce
	functionName := "TestAtlasSupervisorCompletionMarker" + nonce
	body := fmt.Sprintf("package %s\n\nimport (\n\t\"fmt\"\n\t\"testing\"\n)\n\n// %s proves the supervisor-owned test body executed.\nfunc %s(t *testing.T) {\n\tfmt.Println(%q)\n}\n", file.Name.Name, functionName, functionName, marker)
	markerPath := filepath.Join(filepath.Dir(source.path), "atlas_eval_completion_marker_"+nonce+"_test.go")
	if err := session.WriteFile(markerPath, []byte(body)); err != nil {
		return nil, "", err
	}
	arguments = append([]string(nil), arguments...)
	foundRun := false
	for index := 0; index+1 < len(arguments); index++ {
		if arguments[index] == "-run" {
			original := strings.Trim(arguments[index+1], "^$")
			arguments[index+1] = "^(" + original + "|" + functionName + ")$"
			foundRun = true
			break
		}
	}
	if !foundRun {
		arguments = append(arguments, "-run", "^"+functionName+"$")
	}
	arguments = append(arguments, "-v")
	return arguments, marker, nil
}

// summarizeSurfaceChecks produces one endpoint without concealing individual failures.
func summarizeSurfaceChecks(id string, checks []EndpointResult) EndpointResult {
	failed := 0
	ineligible := 0
	for _, check := range checks {
		if check.Kind == RequirementQuality {
			continue
		}
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
