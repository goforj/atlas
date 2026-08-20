package workflows

import (
	"path/filepath"
	"strings"

	"github.com/goforj/atlas/project"
)

// FilePolicyRequest asks Atlas to classify a project path.
type FilePolicyRequest struct {
	Path        string          `json:"path"`
	Task        string          `json:"task,omitempty"`
	Resource    string          `json:"resource,omitempty"`
	WorkflowIDs []string        `json:"workflow_ids,omitempty"`
	Project     project.Project `json:"project,omitempty"`
	ProjectRoot string          `json:"project_root,omitempty"`
	Rules       []OwnershipRule `json:"rules,omitempty"`
}

// OwnershipRule describes a project-specific path ownership override.
type OwnershipRule struct {
	Pattern         string `json:"pattern"`
	Classification  string `json:"classification"`
	Editable        bool   `json:"editable"`
	PreferredAction string `json:"preferred_action,omitempty"`
	ChangeThrough   string `json:"change_through,omitempty"`
	Reason          string `json:"reason,omitempty"`
}

// FilePolicyResult describes how agents should treat a project path.
type FilePolicyResult struct {
	Path            string   `json:"path"`
	Classification  string   `json:"classification"`
	Editable        bool     `json:"editable"`
	PreferredAction string   `json:"preferred_action"`
	ChangeThrough   string   `json:"change_through,omitempty"`
	App             string   `json:"app,omitempty"`
	FrontendKit     string   `json:"frontend_kit,omitempty"`
	Owner           string   `json:"owner,omitempty"`
	MatchedRule     string   `json:"matched_rule,omitempty"`
	Reason          string   `json:"reason"`
	Warnings        []string `json:"warnings,omitempty"`
}

// FilePolicy classifies whether a path is generated, app-owned, or user-owned.
func FilePolicy(req FilePolicyRequest) FilePolicyResult {
	clean := cleanPolicyPath(req.Path)
	if clean == "." || clean == "" {
		return FilePolicyResult{Path: req.Path, Classification: "unknown", Editable: false, PreferredAction: "do not edit", Reason: "path is required"}
	}

	for _, rule := range req.Rules {
		if ownershipRuleMatches(rule.Pattern, clean) {
			return FilePolicyResult{
				Path:            clean,
				Classification:  firstNonEmptyString(rule.Classification, "project-owned"),
				Editable:        rule.Editable,
				PreferredAction: firstNonEmptyString(rule.PreferredAction, actionForEditable(rule.Editable)),
				ChangeThrough:   rule.ChangeThrough,
				Owner:           "project",
				MatchedRule:     rule.Pattern,
				Reason:          firstNonEmptyString(rule.Reason, "Project Atlas ownership override matched this path."),
			}
		}
	}

	appName, appScoped := appForPath(clean)
	namedApp := appName != "" && appName != project.DefaultAppName
	frontendKit := strings.TrimSpace(req.Project.FrontendKit)
	switch {
	case strings.HasSuffix(clean, "wire_gen.go"):
		return FilePolicyResult{Path: clean, Classification: "generated", Editable: false, PreferredAction: "regenerate", ChangeThrough: "forj build", App: appName, Owner: "framework-generator", Reason: "Wire generated output must not be edited by hand.", Warnings: []string{"Fix provider sets, then regenerate with forj build."}}
	case strings.Contains(clean, "/wire/") && strings.HasPrefix(clean, "app/"):
		return FilePolicyResult{Path: clean, Classification: appScopedClassification(namedApp, "wire"), Editable: true, PreferredAction: "direct edit provider set", ChangeThrough: "provider functions and forj build", App: appName, Owner: "app", Reason: "App Wire input files are preserved app composition surfaces.", Warnings: []string{"Do not add nil guards around required constructor dependencies."}}
	case appScoped && isAppRegistrationPath(clean):
		return FilePolicyResult{Path: clean, Classification: appScopedClassification(namedApp, "registration"), Editable: true, PreferredAction: "prefer forj make:*", ChangeThrough: makeCommandForApp(appName), App: appName, Owner: "app", Reason: "App composition files own routes, commands, schedules, lifecycle hooks, and registration."}
	case isFrontendPath(clean):
		return FilePolicyResult{Path: clean, Classification: "frontend-owned", Editable: true, PreferredAction: "direct edit", ChangeThrough: "starter-kit frontend workflow", App: appName, FrontendKit: frontendKit, Owner: "app-frontend", Reason: frontendReason(frontendKit)}
	case strings.HasPrefix(clean, "cmd/"):
		return FilePolicyResult{Path: clean, Classification: "framework-owned-entrypoint", Editable: false, PreferredAction: "do not edit", ChangeThrough: "app composition and runtime wiring", App: appName, Owner: "framework", Reason: "App binary entrypoints should stay thin."}
	case strings.HasPrefix(clean, "internal/runtime") || strings.HasPrefix(clean, "internal/http") || strings.HasPrefix(clean, "internal/jobs") || strings.HasPrefix(clean, "internal/schedules"):
		return FilePolicyResult{Path: clean, Classification: "framework-owned-runtime", Editable: false, PreferredAction: "do not edit", ChangeThrough: "GoForj templates or generated extension points", Owner: "framework", Reason: "Framework runtime packages own bootstrap and lifecycle behavior."}
	case dataGeneratorPolicyApplies(req, clean):
		resource := dataPolicyResource(req, clean)
		return FilePolicyResult{
			Path:            clean,
			Classification:  "generator-first-data",
			Editable:        true,
			PreferredAction: "generate then extend",
			ChangeThrough:   "forj make:migration <name>, forj migrate, then forj make:model " + resource + " --package " + resource,
			Owner:           "app",
			Reason:          "GoForj derives the model, repository shape, and repository Wire registration from the applied schema; application-specific repository methods remain App-owned extensions.",
			Warnings: []string{
				"Never hand-create GORM models or persistence repositories when make:model applies.",
				"Create and apply the migration before running make:model.",
				"Use a domain-native table name unless an existing schema or explicit requirement requires a prefix.",
			},
		}
	case strings.HasPrefix(clean, "internal/"):
		return FilePolicyResult{Path: clean, Classification: "user-owned-domain", Editable: true, PreferredAction: "direct edit", Owner: "user", Reason: "Business behavior belongs in cohesive packages under internal."}
	case strings.HasPrefix(clean, "migrations/"):
		return FilePolicyResult{Path: clean, Classification: "migration-owned", Editable: true, PreferredAction: "prefer forj make:migration", ChangeThrough: "forj make:migration when creating or removing migrations", Owner: "app", Reason: "Migration SQL is application-owned and should remain explicit."}
	case clean == ".goforj.yml" || clean == ".goforj/atlas.json" || strings.HasPrefix(clean, ".env"):
		return FilePolicyResult{Path: clean, Classification: "config-owned", Editable: true, PreferredAction: "direct edit", ChangeThrough: "configuration plus forj build when generated resources change", Owner: "project", Reason: "Configuration selects components, apps, drivers, agent settings, and named resources."}
	case strings.HasPrefix(clean, "docs/"):
		return FilePolicyResult{Path: clean, Classification: "user-owned-docs", Editable: true, PreferredAction: "direct edit", Owner: "user", Reason: "Project documentation is user-owned unless a project override says otherwise."}
	default:
		return FilePolicyResult{Path: clean, Classification: "user-owned", Editable: true, PreferredAction: "direct edit", Owner: "user", Reason: "Atlas has no generated-file rule for this path."}
	}
}

// dataGeneratorPolicyApplies narrows generator-first ownership to persistence creation tasks instead of banning handwritten repositories globally.
func dataGeneratorPolicyApplies(req FilePolicyRequest, path string) bool {
	if !strings.HasPrefix(path, "internal/") {
		return false
	}
	base := strings.ToLower(filepath.Base(path))
	if base != "repository.go" && !strings.Contains(base, "model") {
		return false
	}
	if stringSliceContains(req.WorkflowIDs, "goforj-add-data-resource") {
		return true
	}
	lower := strings.ToLower(req.Task)
	return containsAny(lower, "make:model", "create model", "create repository", "new data resource", "database table", "persist")
}

// dataPolicyResource provides a useful maker command without treating task prose as a schema parser.
func dataPolicyResource(req FilePolicyRequest, path string) string {
	if resource := strings.TrimSpace(req.Resource); resource != "" {
		return resource
	}
	parts := strings.Split(path, "/")
	if len(parts) >= 3 && parts[0] == "internal" && strings.TrimSpace(parts[1]) != "" && parts[1] != "<package>" && parts[1] != "<domain>" {
		return parts[1]
	}
	return "<table>"
}

// FilePolicies classifies a list of planned files with the same ownership model.
func FilePolicies(req FilePolicyRequest, paths []string) []FilePolicyResult {
	out := make([]FilePolicyResult, 0, len(paths))
	for _, path := range paths {
		next := req
		next.Path = path
		out = append(out, FilePolicy(next))
	}
	return out
}

func cleanPolicyPath(path string) string {
	return filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
}

func ownershipRuleMatches(pattern string, path string) bool {
	pattern = filepath.ToSlash(filepath.Clean(strings.TrimSpace(pattern)))
	if pattern == "." || pattern == "" {
		return false
	}
	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "/**")
		return path == prefix || strings.HasPrefix(path, prefix+"/")
	}
	if ok, _ := filepath.Match(pattern, path); ok {
		return true
	}
	return pattern == path
}

func appForPath(path string) (string, bool) {
	parts := strings.Split(path, "/")
	switch {
	case len(parts) >= 1 && parts[0] == "app":
		if len(parts) == 2 && parts[1] != "wire" && !strings.HasSuffix(parts[1], ".go") {
			return parts[1], true
		}
		if len(parts) >= 3 && parts[1] != "wire" && isKnownAppSubpath(parts[2]) {
			return parts[1], true
		}
		return project.DefaultAppName, true
	case len(parts) >= 3 && parts[0] == "cmd":
		return parts[1], true
	default:
		return "", false
	}
}

func isKnownAppSubpath(value string) bool {
	switch value {
	case "wire", "routes.go", "commands.go", "schedules.go", "lifecycle.go":
		return true
	default:
		return false
	}
}

func isAppRegistrationPath(path string) bool {
	return path == "app" || strings.HasPrefix(path, "app/")
}

func isFrontendPath(path string) bool {
	return strings.HasPrefix(path, "cmd/") && strings.Contains(path, "/frontend")
}

func appScopedClassification(named bool, suffix string) string {
	if named {
		return "named-app-owned-" + suffix
	}
	return "app-owned-" + suffix
}

func makeCommandForApp(appName string) string {
	if appName == "" || appName == project.DefaultAppName {
		return "forj make:* when a generator exists"
	}
	return "forj " + appName + " make:* when a generator exists"
}

func frontendReason(frontendKit string) string {
	switch strings.ToLower(strings.TrimSpace(frontendKit)) {
	case "vue":
		return "Vue starter kit frontend files are app-owned after generation."
	case "react":
		return "React starter kit frontend files are app-owned after generation."
	case "templ", "templ-htmx", "htmx":
		return "templ/htmx starter kit UI files are app-owned after generation."
	default:
		return "Starter kit frontend files are app-owned after generation."
	}
}

func actionForEditable(editable bool) string {
	if editable {
		return "direct edit"
	}
	return "do not edit"
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
