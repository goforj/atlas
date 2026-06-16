package agents

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeminiWriteMCPConfigPreservesExistingSettings(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".gemini", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir settings dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"ui":{"theme":"GitHub"}}`), 0o644); err != nil {
		t.Fatalf("write settings: %v", err)
	}

	err := Gemini{}.WriteMCPConfig(context.Background(), root, MCPServerConfig{
		Name:    "goforj-atlas",
		Command: "forj",
		CWD:     root,
	})
	if err != nil {
		t.Fatalf("write mcp config: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	got := string(content)
	for _, want := range []string{`"theme": "GitHub"`, `"mcpServers"`, `"goforj-atlas"`, `"atlas:mcp"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("settings missing %q:\n%s", want, got)
		}
	}
}
