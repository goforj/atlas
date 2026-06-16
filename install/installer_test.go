package install

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
