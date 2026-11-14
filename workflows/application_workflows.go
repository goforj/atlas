package workflows

import (
	"regexp"
	"sort"
	"strings"
)

// applicationWorkflowDefinition keeps focused intent modular; omitted metadata retains established workflow policies.
type applicationWorkflowDefinition struct {
	commands  []string
	id        string
	matches   func(string) bool
	docsTopic func(string) bool
	docs      []DocReference
	files     []string
	steps     []PlanStep
	warnings  []string
}

var applicationWorkflowDefinitions = map[string]applicationWorkflowDefinition{}
var applicationChangePattern = regexp.MustCompile(`\b(add|create|implement|update|change|configure|bind|wire|inject|use|validate|normalize|preserve|distinguish|encrypt|decrypt|rotate|read|resolve|fetch|load|protect|reject|require|introduce|integrate|fix|repair|support|ensure|make|set up|look up|declare|harden|build|improve|send|notify|search|index|target|evaluate|enable|disable|roll out|instrument|increment|record|observe|measure)\b`)
var applicationSurfacePattern = regexp.MustCompile(`\b(add|create|generate|introduce|expose)\s+(?:[a-z0-9_-]+\s+){0,4}(route|endpoint|controller|job|schedule|command)\b`)

// init makes explicit docs requests addressable without turning them into source work.
func init() {
	WorkflowDocsMap["goforj-read-docs"] = []DocReference{{Path: "applications/library-selection.md", Heading: "Match the Application Requirement"}}
}

// registerApplicationWorkflow installs immutable package-owned definitions before planning starts.
func registerApplicationWorkflow(definition applicationWorkflowDefinition) {
	applicationWorkflowDefinitions[definition.id] = definition
	if len(definition.docs) != 0 {
		WorkflowDocsMap[definition.id] = append([]DocReference(nil), definition.docs...)
	}
}

// nonImplementationWorkflow preserves inspection intent before library and generator keyword matching.
func nonImplementationWorkflow(task string) string {
	task = strings.ToLower(strings.TrimSpace(task))
	for _, prefix := range []string{"please ", "can you ", "could you "} {
		task = strings.TrimPrefix(task, prefix)
	}
	switch {
	case hasTaskPrefix(task, "write tests", "add tests", "add regression tests", "write a test", "create a test", "create tests", "add a test"), strings.HasPrefix(task, "go test"), strings.HasPrefix(task, "run tests"), strings.HasPrefix(task, "test "), strings.HasPrefix(task, "verify "), strings.HasPrefix(task, "review "):
		return "goforj-validate-change"
	case strings.HasPrefix(task, "debug "), strings.HasPrefix(task, "diagnose "), strings.HasPrefix(task, "troubleshoot "), strings.HasPrefix(task, "investigate "):
		return "goforj-debug-runtime"
	case strings.HasPrefix(task, "explain "), hasTaskPrefix(task, "search docs", "search documentation", "read docs", "read documentation", "find docs", "show docs", "browse docs"), strings.HasPrefix(task, "look up ") && containsAny(task, "docs", "documentation", "api reference"):
		return "goforj-read-docs"
	default:
		return ""
	}
}

// hasTaskPrefix distinguishes inspection requests from implementation tasks that merely mention reading docs.
func hasTaskPrefix(task string, prefixes ...string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(task, prefix) {
			return true
		}
	}
	return false
}

// applicationWorkflowIDs keeps library routing deterministic and preserves genuine Wire repair requests.
func applicationWorkflowIDs(task string, requireChange bool) []string {
	if requireChange && (nonImplementationWorkflow(task) != "" || !applicationChangePattern.MatchString(task) || containsAny(task, "no provider found", "missing provider", "duplicate provider", "multiple providers", "wire error")) {
		return nil
	}
	ids := make([]string, 0, len(applicationWorkflowDefinitions))
	for id, definition := range applicationWorkflowDefinitions {
		matches := definition.docsTopic(task)
		if requireChange {
			matches = definition.matches(task)
		}
		if matches {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

// applicationWorkflows adds only explicitly requested new surfaces to an application-owned change.
func applicationWorkflows(task string) []string {
	ids := applicationWorkflowIDs(task, true)
	if len(ids) == 0 {
		return nil
	}
	for _, match := range applicationSurfacePattern.FindAllStringSubmatch(task, -1) {
		if containsAny(" "+match[0]+" ", " existing ", " to ", " in ", " for ", " on ") {
			continue
		}
		id := ""
		switch match[2] {
		case "route", "endpoint", "controller":
			id = "goforj-add-http-route"
		case "job":
			id = "goforj-add-job"
		case "schedule":
			id = "goforj-add-schedule"
		case "command":
			id = "goforj-add-app-command"
		}
		ids = appendUniqueStrings(ids, id)
	}
	return ids
}

// applicationWorkflowCommands scopes declared commands and retains existing policies when metadata is omitted.
func applicationWorkflowCommands(id string, scope appScope) ([]string, bool) {
	if id == "goforj-read-docs" {
		return nil, true
	}
	definition, ok := applicationWorkflowDefinitions[id]
	if !ok || len(definition.commands) == 0 {
		return nil, false
	}
	return applicationCommandTemplates(definition.commands, scope), true
}

// applicationCommandTemplates scopes real declared commands without creating generators from task nouns.
func applicationCommandTemplates(commands []string, scope appScope) []string {
	result := make([]string, len(commands))
	replacer := strings.NewReplacer("{forj}", strings.TrimSpace(scope.command("")), "{build}", scope.build)
	for index, command := range commands {
		result[index] = replacer.Replace(command)
	}
	return result
}

// applicationWorkflowFiles resolves selected-App provider inputs without exposing generated Wire output.
func applicationWorkflowFiles(id string, scope layout) ([]string, bool) {
	if id == "goforj-read-docs" {
		return nil, true
	}
	definition, ok := applicationWorkflowDefinitions[id]
	if !ok || len(definition.files) == 0 {
		return nil, false
	}
	files := make([]string, len(definition.files))
	for index, path := range definition.files {
		files[index] = strings.NewReplacer("{wire}", scope.wire, "{composition}", scope.composition).Replace(path)
	}
	return files, true
}

// applicationWorkflowSteps copies definition steps because Plan assigns ordering for each request.
func applicationWorkflowSteps(id string, scope appScope) ([]PlanStep, bool) {
	definition, ok := applicationWorkflowDefinitions[id]
	if !ok || len(definition.steps) == 0 {
		return nil, false
	}
	steps := append([]PlanStep(nil), definition.steps...)
	for index := range steps {
		steps[index].WorkflowID = id
		steps[index].Action = applicationCommandTemplates([]string{steps[index].Action}, scope)[0]
	}
	steps = append(steps, PlanStep{WorkflowID: id, Action: "go test ./...", Purpose: "prove application behavior and failure boundaries with focused regression tests"})
	return steps, true
}

// applicationWorkflowDocs keeps read-only discovery focused on the requested topic and valid existing docs.
func applicationWorkflowDocs(id, task string) []DocReference {
	if id != "goforj-read-docs" {
		return WorkflowDocsMap[id]
	}
	refs := []DocReference{}
	for _, workflowID := range applicationWorkflowIDs(strings.ToLower(task), false) {
		for _, ref := range WorkflowDocsMap[workflowID] {
			if !docRefsContain(refs, ref.Path) {
				refs = append(refs, ref)
			}
		}
	}
	if strings.Contains(strings.ToLower(task), "wire") {
		refs = append(refs, DocReference{Path: "developer-tools/reading-wire-errors.md", Heading: "Fast Checklist"})
	}
	if len(refs) == 0 {
		return WorkflowDocsMap[id]
	}
	return refs
}

// applicationDocsPlan returns retrieval steps without recommending source edits or build side effects.
func applicationDocsPlan(app, task string) PlanResult {
	return PlanResult{
		WorkflowID: "goforj-read-docs", WorkflowIDs: []string{"goforj-read-docs"}, App: app,
		Tools: []string{"search-docs", "read-doc-section", "docs-section-pack"}, Docs: applicationWorkflowDocs("goforj-read-docs", task),
		Steps: []PlanStep{{Order: 1, WorkflowID: "goforj-read-docs", Action: "Atlas search-docs and read-doc-section", Purpose: "read the requested documentation before proposing implementation work"}},
	}
}

// appendUniquePostBuildStep avoids repeating shared verification after multiple explicit application concerns.
func appendUniquePostBuildStep(steps []PlanStep, step PlanStep) []PlanStep {
	for _, existing := range steps {
		if existing.Action == step.Action {
			return steps
		}
	}
	return append(steps, step)
}
