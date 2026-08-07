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

func TestJSONAdaptersPreserveUnrelatedMCPServers(t *testing.T) {
	tests := []struct {
		name  string
		agent Agent
		key   string
	}{
		{name: "claude", agent: Claude{}, key: "mcpServers"},
		{name: "copilot", agent: Copilot{}, key: "servers"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			path := test.agent.MCPConfigPath(root)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatalf("mkdir config dir: %v", err)
			}
			fixture := `{"theme":"dark","` + test.key + `":{"other":{"command":"other"}}}`
			if err := os.WriteFile(path, []byte(fixture), 0o644); err != nil {
				t.Fatalf("write fixture: %v", err)
			}
			server := MCPServerConfig{Name: "goforj-atlas", Command: "forj", CWD: "."}
			if err := test.agent.WriteMCPConfig(context.Background(), root, server); err != nil {
				t.Fatalf("write MCP config: %v", err)
			}
			remover := test.agent.(MCPConfigRemover)
			if err := remover.RemoveMCPConfig(context.Background(), root, server.Name); err != nil {
				t.Fatalf("remove MCP config: %v", err)
			}
			got := string(mustReadAgentFile(t, path))
			for _, want := range []string{`"theme": "dark"`, `"other"`, `"command": "other"`} {
				if !strings.Contains(got, want) {
					t.Fatalf("config missing %q after merge and removal:\n%s", want, got)
				}
			}
			if strings.Contains(got, "goforj-atlas") {
				t.Fatalf("Atlas server remained after removal:\n%s", got)
			}
		})
	}
}

func TestCodexMCPConfigPreservesUnrelatedTables(t *testing.T) {
	root := t.TempDir()
	path := Codex{}.MCPConfigPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	fixture := "# keep this\n[features]\nweb_search = true\n\n[mcp_servers.other]\ncommand = \"other\"\n"
	if err := os.WriteFile(path, []byte(fixture), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	server := MCPServerConfig{Name: "goforj-atlas", Command: "forj", CWD: "."}
	if err := (Codex{}).WriteMCPConfig(context.Background(), root, server); err != nil {
		t.Fatalf("write MCP config: %v", err)
	}
	if err := (Codex{}).RemoveMCPConfig(context.Background(), root, server.Name); err != nil {
		t.Fatalf("remove MCP config: %v", err)
	}
	got := string(mustReadAgentFile(t, path))
	for _, want := range []string{"# keep this", "[features]", "web_search = true", "[mcp_servers.other]"} {
		if !strings.Contains(got, want) {
			t.Fatalf("config missing %q after merge and removal:\n%s", want, got)
		}
	}
	if strings.Contains(got, "goforj-atlas") {
		t.Fatalf("Atlas table remained after removal:\n%s", got)
	}
}

// mustReadAgentFile reads a test fixture or stops the test at the point of failure.
func mustReadAgentFile(t *testing.T, path string) []byte {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return content
}
