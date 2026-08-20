package mcp

import (
	"context"
	"fmt"

	"github.com/goforj/atlas/config"
	"github.com/goforj/atlas/workflows"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

// workflowPlan returns the deterministic framework workflow for a task.
func (s Server) workflowPlan(ctx context.Context, request mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	task, err := request.RequireString("task")
	if err != nil {
		return toolError(err)
	}
	result, ok := workflows.Plan(s.workflowContext(ctx), workflows.PlanRequest{
		Task: task,
		App:  appName(request),
	})
	if !ok {
		return toolError(fmt.Errorf("app not found: %s", appName(request)))
	}
	return jsonResult(result)
}

// registrationPoints returns app-owned registration and Wire files.
func (s Server) registrationPoints(_ context.Context, request mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	result, ok := workflows.RegistrationPoints(s.projectWithDefaults(), appName(request))
	if !ok {
		return toolError(fmt.Errorf("app not found: %s", appName(request)))
	}
	return jsonResult(result)
}

// validationPlan returns the checks agents should run for a workflow.
func (s Server) validationPlan(ctx context.Context, request mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	task, err := request.RequireString("task")
	if err != nil {
		return toolError(err)
	}
	p := s.projectWithDefaults()
	if _, ok := p.AppByName(appName(request)); !ok {
		return toolError(fmt.Errorf("app not found: %s", appName(request)))
	}
	return jsonResult(workflows.ValidationPlan(s.workflowContext(ctx), workflows.PlanRequest{
		Task: task,
		App:  appName(request),
	}))
}

// wireDiagnostics classifies Wire output without running any build command.
func (s Server) wireDiagnostics(_ context.Context, request mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	output, err := request.RequireString("output")
	if err != nil {
		return toolError(err)
	}
	return jsonResult(workflows.DiagnoseWire(output))
}

// scenarioGuide returns verified scenario references from the docs provider.
func (s Server) scenarioGuide(ctx context.Context, request mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	query, err := request.RequireString("query")
	if err != nil {
		return toolError(err)
	}
	result, err := workflows.ScenarioGuide(_context(ctx), s.docsProvider(), query)
	if err != nil {
		return nil, err
	}
	return jsonResult(result)
}

// resourceInventory returns visible app and runtime resources.
func (s Server) resourceInventory(ctx context.Context, _ mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	return jsonResult(workflows.Resources(s.workflowContext(ctx)))
}

// generatedFilePolicy classifies a path before an agent edits it.
func (s Server) generatedFilePolicy(_ context.Context, request mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	path, err := request.RequireString("path")
	if err != nil {
		return toolError(err)
	}
	return jsonResult(workflows.FilePolicy(workflows.FilePolicyRequest{
		Path:        path,
		Task:        request.GetString("task", ""),
		Resource:    request.GetString("resource", ""),
		WorkflowIDs: workflows.ClassifyAll(request.GetString("task", "")),
		Project:     s.projectWithDefaults(),
		Rules:       s.ownershipRules(),
	}))
}

// commandAdvice returns the preferred GoForj command for a task.
func (s Server) commandAdvice(ctx context.Context, request mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	task, err := request.RequireString("task")
	if err != nil {
		return toolError(err)
	}
	result, ok := workflows.CommandAdvice(s.workflowContext(ctx), workflows.CommandAdviceRequest{
		Task:     task,
		App:      appName(request),
		Resource: request.GetString("resource", ""),
	})
	if !ok {
		return toolError(fmt.Errorf("app not found: %s", appName(request)))
	}
	return jsonResult(result)
}

// docsSectionPack returns workflow docs sections in reading order.
func (s Server) docsSectionPack(ctx context.Context, request mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	result, err := workflows.DocsSectionPack(
		_context(ctx),
		s.docsProvider(),
		request.GetString("workflow_id", ""),
		request.GetString("task", ""),
		request.GetInt("token_limit", 120),
	)
	if err != nil {
		return nil, err
	}
	alignment := s.versionAlignmentResult(ctx)
	result.Alignment = &alignment
	return jsonResult(result)
}

// workflowScorecard returns deterministic workflow fixture results.
func (s Server) workflowScorecard(ctx context.Context, request mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	return jsonResult(workflows.RunEvalFixtures(s.workflowContext(ctx), request.GetBool("capture_transcript", false)))
}

// versionAlignment compares active docs metadata with project and Atlas versions.
func (s Server) versionAlignment(ctx context.Context, _ mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	return jsonResult(s.versionAlignmentResult(ctx))
}

func (s Server) versionAlignmentResult(ctx context.Context) workflows.VersionAlignmentResult {
	manifest, _ := s.docsProvider().Manifest(_context(ctx))
	return workflows.VersionAlignment(s.projectWithDefaults(), s.Version, manifest)
}

func (s Server) ownershipRules() []workflows.OwnershipRule {
	if s.Project.Root == "" {
		return nil
	}
	cfg, err := config.Load(s.Project.Root)
	if err != nil {
		return nil
	}
	rules := make([]workflows.OwnershipRule, 0, len(cfg.OwnershipRules))
	for _, rule := range cfg.OwnershipRules {
		rules = append(rules, workflows.OwnershipRule{
			Pattern:         rule.Pattern,
			Classification:  rule.Classification,
			Editable:        rule.Editable,
			PreferredAction: rule.PreferredAction,
			ChangeThrough:   rule.ChangeThrough,
			Reason:          rule.Reason,
		})
	}
	return rules
}
