package workflows

import (
	"strings"

	"github.com/goforj/atlas/diagnostics"
)

// RuntimeSnapshotRequest describes local runtime evidence the agent wants to inspect.
type RuntimeSnapshotRequest struct {
	App        string `json:"app,omitempty"`
	Runtime    string `json:"runtime,omitempty"`
	Path       string `json:"path,omitempty"`
	RouteName  string `json:"route_name,omitempty"`
	TimeWindow string `json:"time_window,omitempty"`
	Limit      int    `json:"limit,omitempty"`
}

// RuntimeSnapshotResult combines safe local evidence for one app/runtime.
type RuntimeSnapshotResult struct {
	App             string                        `json:"app"`
	Runtime         string                        `json:"runtime,omitempty"`
	Path            string                        `json:"path,omitempty"`
	RouteName       string                        `json:"route_name,omitempty"`
	TimeWindow      string                        `json:"time_window,omitempty"`
	Routes          []string                      `json:"routes,omitempty"`
	Resources       []ResourceLink                `json:"resources,omitempty"`
	Logs            []diagnostics.LogEntry        `json:"logs,omitempty"`
	LastError       diagnostics.LogEntry          `json:"last_error,omitempty"`
	LastErrorFound  bool                          `json:"last_error_found"`
	AbsoluteURL     string                        `json:"absolute_url,omitempty"`
	BrowserLogs     []diagnostics.BrowserLogEntry `json:"browser_logs,omitempty"`
	Metrics         diagnostics.MetricsMetadata   `json:"metrics,omitempty"`
	MissingEvidence []string                      `json:"missing_evidence,omitempty"`
	Confidence      string                        `json:"confidence"`
}

// DebugPlanResult recommends read-only next steps from a runtime snapshot.
type DebugPlanResult struct {
	App             string          `json:"app"`
	Runtime         string          `json:"runtime,omitempty"`
	Path            string          `json:"path,omitempty"`
	RouteName       string          `json:"route_name,omitempty"`
	Confidence      string          `json:"confidence"`
	Steps           []DebugPlanStep `json:"steps"`
	MissingEvidence []string        `json:"missing_evidence,omitempty"`
}

// DebugPlanStep describes one read-only inspection step.
type DebugPlanStep struct {
	Tool     string `json:"tool"`
	Purpose  string `json:"purpose"`
	Required bool   `json:"required"`
}

// DebugPlan builds next read-only inspection steps from available runtime evidence.
func DebugPlan(snapshot RuntimeSnapshotResult) DebugPlanResult {
	steps := []DebugPlanStep{
		{Tool: "application-info", Purpose: "confirm app and runtime identity before changing code", Required: true},
		{Tool: "resource-inventory", Purpose: "find local app, operator, and observability links", Required: true},
	}
	if snapshot.Path != "" || snapshot.RouteName != "" {
		steps = append(steps,
			DebugPlanStep{Tool: "route-list", Purpose: "confirm the route is registered on the selected app", Required: true},
			DebugPlanStep{Tool: "get-absolute-url", Purpose: "resolve the local URL instead of guessing a port", Required: true},
		)
	}
	steps = append(steps,
		DebugPlanStep{Tool: "read-log-entries", Purpose: "inspect recent app/runtime logs in the requested time window", Required: true},
		DebugPlanStep{Tool: "last-error", Purpose: "anchor investigation on the latest error-level log", Required: false},
		DebugPlanStep{Tool: "browser-logs", Purpose: "check client-side failures when the issue has a browser surface", Required: false},
		DebugPlanStep{Tool: "metrics-metadata", Purpose: "confirm labels and scrape targets before using metrics", Required: false},
	)
	return DebugPlanResult{
		App:             snapshot.App,
		Runtime:         snapshot.Runtime,
		Path:            snapshot.Path,
		RouteName:       snapshot.RouteName,
		Confidence:      snapshot.Confidence,
		Steps:           steps,
		MissingEvidence: append([]string(nil), snapshot.MissingEvidence...),
	}
}

// ConfidenceForRuntimeEvidence summarizes how complete a runtime snapshot is.
func ConfidenceForRuntimeEvidence(snapshot RuntimeSnapshotResult) string {
	missing := len(snapshot.MissingEvidence)
	switch {
	case missing == 0:
		return "high"
	case missing <= 2:
		return "medium"
	default:
		return "low"
	}
}

// FilterResourcesForRuntime returns links that are useful while debugging a runtime.
func FilterResourcesForRuntime(resources []ResourceLink, app string, runtime string) []ResourceLink {
	out := []ResourceLink{}
	for _, resource := range resources {
		category := strings.ToLower(resource.Category)
		if category == "app" || category == "api" || category == "observability" || category == "operator" || category == "docs" {
			out = append(out, resource)
		}
	}
	return out
}
