package install

import (
	"context"
	"os"
	"path/filepath"
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

	assertContains(t, filepath.Join(root, "AGENTS.md"), "cmd/app/main.go")
	assertContains(t, filepath.Join(root, "CLAUDE.md"), "forj make:*")
	assertContains(t, filepath.Join(root, "GEMINI.md"), "GoForj Atlas")
	assertContains(t, filepath.Join(root, ".github", "copilot-instructions.md"), "internal/")
	assertContains(t, filepath.Join(root, ".codex", "config.toml"), "atlas:mcp")
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
	if !strings.Contains(content, "Do not delete me.") {
		t.Fatalf("user content was not preserved:\n%s", content)
	}
	if strings.Count(content, "<!-- goforj-atlas:start -->") != 1 {
		t.Fatalf("expected one marker block:\n%s", content)
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
	content := readFile(t, path) + "\n\n# Local Rules\n\nKeep this.\n"
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
	if got := readFile(t, path); !strings.Contains(got, "Keep this.") {
		t.Fatalf("user content was not preserved:\n%s", got)
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
