package install

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/goforj/atlas/agents"
	"github.com/goforj/atlas/config"
	atlasdocs "github.com/goforj/atlas/docs"
	"github.com/goforj/atlas/project"
)

func TestInstallerWritesAgentFilesAndConfig(t *testing.T) {
	root := t.TempDir()
	installer := NewInstaller()

	result, err := installer.Install(context.Background(), Options{
		Root:      root,
		AllAgents: true,
		Project: project.Project{
			Root:          root,
			Name:          "demo",
			GoForjVersion: "0.18.0",
			Components:    []string{"web-api", "jobs"},
		},
	})
	if err != nil {
		t.Fatalf("install failed: %v", err)
	}
	if len(result.Agents) != 4 {
		t.Fatalf("expected 4 agents, got %d", len(result.Agents))
	}

	for _, path := range []string{"AGENTS.md", "CLAUDE.md", "GEMINI.md", filepath.Join(".github", "copilot-instructions.md")} {
		assertContains(t, filepath.Join(root, path), "<!-- goforj-atlas:start -->")
	}
	assertContains(t, filepath.Join(root, ".codex", "config.toml"), "atlas:mcp")
	assertContains(t, filepath.Join(root, ".codex", "config.toml"), "required = true")
	assertContains(t, filepath.Join(root, ".mcp.json"), "goforj-atlas")
	assertContains(t, filepath.Join(root, ".vscode", "mcp.json"), "goforj-atlas")
	assertContains(t, filepath.Join(root, ".gemini", "settings.json"), "goforj-atlas")
	assertContains(t, filepath.Join(root, ".goforj", "atlas.json"), `"agents"`)
}

func TestInstallerPreservesUserGuidelineContent(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("# Local Rules\n\nDo not delete me.\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	_, err := NewInstaller().Install(context.Background(), Options{
		Root:   root,
		Agents: []string{"codex"},
		Project: project.Project{
			Root: root,
			Name: "demo",
		},
	})
	if err != nil {
		t.Fatalf("install failed: %v", err)
	}

	content := readFile(t, filepath.Join(root, "AGENTS.md"))
	if !strings.Contains(content, "# Local Rules\n\nDo not delete me.\n") || !strings.Contains(content, "<!-- goforj-atlas:start -->") {
		t.Fatalf("Atlas did not preserve user content and add its marker:\n%s", content)
	}
}

func TestInstallerDryRunReportsFilesWithoutWriting(t *testing.T) {
	root := t.TempDir()
	result, err := NewInstaller().Install(context.Background(), Options{
		Root:   root,
		Agents: []string{"codex"},
		Project: project.Project{
			Root: root,
			Name: "demo",
		},
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("dry run install failed: %v", err)
	}
	if len(result.Files) == 0 {
		t.Fatalf("expected planned files: %#v", result)
	}
	if _, err := os.Stat(filepath.Join(root, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatalf("dry run wrote AGENTS.md: %v", err)
	}
	if _, err := os.Stat(config.FilePath(root)); !os.IsNotExist(err) {
		t.Fatalf("dry run wrote atlas config: %v", err)
	}
}

func TestHostInstallerReturnsGuidanceIntentWithoutWritingNativeFiles(t *testing.T) {
	root := t.TempDir()
	result, err := NewHostInstaller().Install(context.Background(), HostRequest{
		Root:    root,
		Agents:  []string{"codex"},
		Project: project.Project{Root: root, Name: "demo"},
	})
	if err != nil {
		t.Fatalf("host install failed: %v", err)
	}
	if result.Guidance.Version != GuidanceReconciliationVersion || !result.Guidance.Enabled || !slices.Equal(result.Guidance.Targets, []string{"codex"}) {
		t.Fatalf("guidance = %#v", result.Guidance)
	}
	if _, err := os.Stat(filepath.Join(root, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatalf("host install wrote native guidance: %v", err)
	}
	if _, err := os.Stat(config.FilePath(root)); err != nil {
		t.Fatalf("host install did not write Atlas state: %v", err)
	}
}

func TestHostUpdaterDisablesGuidanceWithoutRemovingNativeMarker(t *testing.T) {
	root := t.TempDir()
	projectInfo := project.Project{Root: root, Name: "demo"}
	if _, err := NewInstaller().Install(context.Background(), Options{Root: root, Agents: []string{"codex"}, Project: projectInfo}); err != nil {
		t.Fatalf("legacy install failed: %v", err)
	}
	path := filepath.Join(root, "AGENTS.md")
	before := readFile(t, path)
	disabled := false
	result, err := NewHostUpdater().Update(context.Background(), HostRequest{
		Root: root, Project: projectInfo, Guidelines: &disabled,
	})
	if err != nil {
		t.Fatalf("host update failed: %v", err)
	}
	if result.Guidance.Enabled {
		t.Fatalf("guidance = %#v, want disabled", result.Guidance)
	}
	if got := readFile(t, path); got != before {
		t.Fatalf("host update changed native marker:\n%s", got)
	}
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Features.Guidelines {
		t.Fatalf("guidelines remain enabled: %#v", cfg.Features)
	}
}

// TestHostUpdaterChangesAgentsWithoutMutatingNativeGuidance keeps projection ownership at the GoForj boundary.
func TestHostUpdaterChangesAgentsWithoutMutatingNativeGuidance(t *testing.T) {
	root := t.TempDir()
	projectInfo := project.Project{Root: root, Name: "demo"}
	if _, err := NewInstaller().Install(context.Background(), Options{Root: root, Agents: []string{"codex"}, Project: projectInfo}); err != nil {
		t.Fatalf("legacy install failed: %v", err)
	}
	path := filepath.Join(root, "AGENTS.md")
	before := readFile(t, path)
	result, err := NewHostUpdater().Update(context.Background(), HostRequest{
		Root: root, Project: projectInfo, Agents: []string{"claude"},
	})
	if err != nil {
		t.Fatalf("host update failed: %v", err)
	}
	if got := readFile(t, path); got != before {
		t.Fatalf("host update changed deselected native guidance:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(root, "CLAUDE.md")); !os.IsNotExist(err) {
		t.Fatalf("host update projected selected native guidance: %v", err)
	}
	if !slices.Equal(result.Guidance.Targets, []string{"claude"}) {
		t.Fatalf("guidance targets = %#v", result.Guidance.Targets)
	}
}

// TestStatusWarnsWhenLegacyGuidanceIsMissing preserves the direct installer health contract.
func TestStatusWarnsWhenLegacyGuidanceIsMissing(t *testing.T) {
	root := t.TempDir()
	projectInfo := project.Project{Root: root, Name: "demo"}
	if _, err := NewInstaller().Install(context.Background(), Options{Root: root, Agents: []string{"codex"}, Project: projectInfo}); err != nil {
		t.Fatalf("install failed: %v", err)
	}
	if err := os.Remove(filepath.Join(root, "AGENTS.md")); err != nil {
		t.Fatal(err)
	}
	status, err := Status(context.Background(), StatusOptions{Root: root, Project: projectInfo})
	if err != nil {
		t.Fatalf("status failed: %v", err)
	}
	if len(status.Warnings) == 0 || status.Agents[0].GuidelinesPresent {
		t.Fatalf("missing guidance status = %#v", status)
	}
}

func TestStatusReportsInstalledAndStaleSkills(t *testing.T) {
	root := t.TempDir()
	_, err := NewInstaller(agents.Codex{}).Install(context.Background(), Options{
		Root:   root,
		Agents: []string{"codex"},
		Project: project.Project{
			Root:          root,
			Name:          "demo",
			GoForjVersion: "0.18.0",
		},
	})
	if err != nil {
		t.Fatalf("install failed: %v", err)
	}
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.Skills = []string{"old-skill"}
	if err := config.Save(root, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	status, err := Status(context.Background(), StatusOptions{
		Root:    root,
		Project: project.Project{Root: root, Name: "demo", GoForjVersion: "0.18.0"},
		Docs:    atlasdocs.StaticProvider{DocsMeta: atlasdocs.Manifest{Version: "0.18.0", Revision: "rev1"}},
		Agents:  []agents.Agent{agents.Codex{}},
	})
	if err != nil {
		t.Fatalf("status failed: %v", err)
	}
	if !status.Installed || status.Project != "demo" || status.GoForjVersion != "0.18.0" || status.DocsRevision != "rev1" {
		t.Fatalf("status = %#v", status)
	}
	if len(status.Agents) != 1 || !status.Agents[0].Configured || !status.Agents[0].MCPPresent || !status.Agents[0].GuidelinesPresent {
		t.Fatalf("agents = %#v", status.Agents)
	}
	if !status.Skills.Stale || len(status.Warnings) == 0 {
		t.Fatalf("expected stale skill warning: %#v", status)
	}
}

func TestUpdaterPreservesUserGuidelineContent(t *testing.T) {
	root := t.TempDir()
	if _, err := NewInstaller(agents.Codex{}).Install(context.Background(), Options{
		Root:   root,
		Agents: []string{"codex"},
		Project: project.Project{
			Root: root,
			Name: "demo",
		},
	}); err != nil {
		t.Fatalf("install failed: %v", err)
	}
	path := filepath.Join(root, "AGENTS.md")
	content := "# Local Rules\n\nKeep this.\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write local content: %v", err)
	}
	if _, err := NewUpdater().Update(context.Background(), Options{
		Root:   root,
		Agents: []string{"codex"},
		Project: project.Project{
			Root: root,
			Name: "demo",
		},
	}); err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if got := readFile(t, path); !strings.Contains(got, content) || !strings.Contains(got, "<!-- goforj-atlas:start -->") {
		t.Fatalf("Atlas did not preserve user content and add its marker:\n%s", got)
	}
}

func TestInstallerSelectsOnePreferredSystemAgent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, dir := range []string{".codex", ".claude"} {
		if err := os.MkdirAll(filepath.Join(home, dir), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	root := t.TempDir()
	result, err := NewInstaller(agents.Codex{}, agents.Claude{}).Install(context.Background(), Options{
		Root:    root,
		Project: project.Project{Root: root, Name: "demo"},
	})
	if err != nil {
		t.Fatalf("install failed: %v", err)
	}
	if len(result.Agents) != 1 || result.Agents[0] != "codex" {
		t.Fatalf("selected agents = %v, want [codex]", result.Agents)
	}
	if _, err := os.Stat(filepath.Join(root, "CLAUDE.md")); !os.IsNotExist(err) {
		t.Fatalf("unexpected Claude projection: %v", err)
	}
}

func TestUpdaterUsesCommittedAgentSelection(t *testing.T) {
	root := t.TempDir()
	projectInfo := project.Project{Root: root, Name: "demo"}
	if _, err := NewInstaller().Install(context.Background(), Options{Root: root, Agents: []string{"codex"}, Project: projectInfo}); err != nil {
		t.Fatalf("install Codex: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("# User Claude Rules\n"), 0o644); err != nil {
		t.Fatalf("write Claude fixture: %v", err)
	}
	result, err := NewUpdater().Update(context.Background(), Options{Root: root, Project: projectInfo})
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if len(result.Agents) != 1 || result.Agents[0] != "codex" {
		t.Fatalf("updated agents = %v, want committed [codex]", result.Agents)
	}
	if got := readFile(t, filepath.Join(root, "CLAUDE.md")); got != "# User Claude Rules\n" {
		t.Fatalf("unconfigured Claude file changed: %q", got)
	}
}

func TestUpdaterRefreshesOneSurfaceWithoutDisablingOthers(t *testing.T) {
	root := t.TempDir()
	projectInfo := project.Project{Root: root, Name: "demo"}
	if _, err := NewInstaller().Install(context.Background(), Options{Root: root, Agents: []string{"codex"}, Project: projectInfo}); err != nil {
		t.Fatalf("install Codex: %v", err)
	}
	if _, err := NewUpdater().Update(context.Background(), Options{Root: root, Guidelines: true, Project: projectInfo}); err != nil {
		t.Fatalf("update guidelines: %v", err)
	}
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if !cfg.Features.Guidelines || !cfg.Features.Skills || !cfg.Features.MCP {
		t.Fatalf("focused update disabled configured surfaces: %#v", cfg.Features)
	}
	for _, path := range []string{filepath.Join(root, ".agents", "skills"), filepath.Join(root, ".codex", "config.toml")} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("configured surface disappeared at %s: %v", path, err)
		}
	}
}

func TestHostUpdaterAppliesExplicitSurfaceDisable(t *testing.T) {
	root := t.TempDir()
	projectInfo := project.Project{Root: root, Name: "demo"}
	if _, err := NewInstaller().Install(context.Background(), Options{Root: root, Agents: []string{"codex"}, Project: projectInfo}); err != nil {
		t.Fatalf("install Codex: %v", err)
	}
	disabled := false
	result, err := NewHostUpdater().Update(context.Background(), HostRequest{
		Root:       root,
		Project:    projectInfo,
		Guidelines: &disabled,
	})
	if err != nil {
		t.Fatalf("disable guidelines: %v", err)
	}
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Features.Guidelines || !cfg.Features.Skills || !cfg.Features.MCP {
		t.Fatalf("features = %#v", cfg.Features)
	}
	if result.Guidance.Enabled {
		t.Fatalf("guidance reconciliation = %#v", result.Guidance)
	}
}

func TestUpdaterDiscoverReplacesCommittedAgentSelection(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := t.TempDir()
	projectInfo := project.Project{Root: root, Name: "demo"}
	if _, err := NewInstaller().Install(context.Background(), Options{Root: root, Agents: []string{"codex"}, Project: projectInfo}); err != nil {
		t.Fatalf("install Codex: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatalf("mkdir Claude home: %v", err)
	}
	result, err := NewUpdater().Update(context.Background(), Options{Root: root, Discover: true, Project: projectInfo})
	if err != nil {
		t.Fatalf("discover update: %v", err)
	}
	if len(result.Agents) != 1 || result.Agents[0] != "claude" {
		t.Fatalf("discovered agents = %v, want [claude]", result.Agents)
	}
}

func TestInstallerPrunesDeselectedAgentProjections(t *testing.T) {
	root := t.TempDir()
	projectInfo := project.Project{Root: root, Name: "demo"}
	if _, err := NewInstaller().Install(context.Background(), Options{Root: root, AllAgents: true, Project: projectInfo}); err != nil {
		t.Fatalf("install all agents: %v", err)
	}
	claudePath := filepath.Join(root, "CLAUDE.md")
	if err := os.WriteFile(claudePath, []byte("# User Rules\n\nKeep me.\n"), 0o644); err != nil {
		t.Fatalf("append Claude rules: %v", err)
	}
	if _, err := NewInstaller().Install(context.Background(), Options{Root: root, Agents: []string{"codex"}, Project: projectInfo}); err != nil {
		t.Fatalf("reinstall Codex: %v", err)
	}
	claude := readFile(t, claudePath)
	if claude != "# User Rules\n\nKeep me.\n" {
		t.Fatalf("Claude user content was not preserved cleanly:\n%s", claude)
	}
	for _, path := range []string{filepath.Join(root, ".claude"), filepath.Join(root, ".gemini"), filepath.Join(root, ".vscode"), filepath.Join(root, "GEMINI.md"), filepath.Join(root, ".mcp.json")} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("deselected projection remains at %s: %v", path, err)
		}
	}
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Version != config.CurrentVersion || len(cfg.Agents) != 1 || cfg.Agents[0] != "codex" {
		t.Fatalf("unexpected config after pruning: %#v", cfg)
	}
}

func TestInstallerPrunesDisabledSurfaces(t *testing.T) {
	root := t.TempDir()
	projectInfo := project.Project{Root: root, Name: "demo"}
	if _, err := NewInstaller().Install(context.Background(), Options{Root: root, Agents: []string{"codex"}, Project: projectInfo}); err != nil {
		t.Fatalf("install Codex: %v", err)
	}
	if _, err := NewInstaller().Install(context.Background(), Options{Root: root, Agents: []string{"codex"}, Guidelines: true, Project: projectInfo}); err != nil {
		t.Fatalf("reinstall guidelines only: %v", err)
	}
	for _, path := range []string{filepath.Join(root, ".agents"), filepath.Join(root, ".codex")} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("disabled surface remains at %s: %v", path, err)
		}
	}
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if !cfg.Features.Guidelines || cfg.Features.Skills || cfg.Features.MCP {
		t.Fatalf("unexpected configured surfaces: %#v", cfg.Features)
	}
	if !slices.Equal(cfg.GeneratedFiles["codex"], []string{"AGENTS.md"}) {
		t.Fatalf("guideline ownership missing from manifest: %#v", cfg.GeneratedFiles)
	}
	status, err := Status(context.Background(), StatusOptions{Root: root, Project: projectInfo})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if len(status.Warnings) != 0 || status.Skills.Expected != 0 || status.Skills.Stale {
		t.Fatalf("doctor warned about intentionally disabled surfaces: %#v", status)
	}
}

func TestStatusChecksSkillsForEveryConfiguredAgent(t *testing.T) {
	root := t.TempDir()
	projectInfo := project.Project{Root: root, Name: "demo"}
	if _, err := NewInstaller().Install(context.Background(), Options{Root: root, Agents: []string{"codex", "claude"}, Project: projectInfo}); err != nil {
		t.Fatalf("install agents: %v", err)
	}
	missing := filepath.Join(root, ".claude", "skills", "goforj-make-commands", "SKILL.md")
	if err := os.Remove(missing); err != nil {
		t.Fatalf("remove fixture skill: %v", err)
	}
	status, err := Status(context.Background(), StatusOptions{Root: root, Project: projectInfo})
	if err != nil {
		t.Fatalf("status failed: %v", err)
	}
	if !status.Skills.Stale || !slices.Contains(status.Skills.Missing, "claude/goforj-make-commands") {
		t.Fatalf("status did not report second-agent skill gap: %#v", status.Skills)
	}
}

func assertContains(t *testing.T, path string, want string) {
	t.Helper()
	content := readFile(t, path)
	if !strings.Contains(content, want) {
		t.Fatalf("%s did not contain %q:\n%s", path, want, content)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}
