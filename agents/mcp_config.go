package agents

import (
	"encoding/json"
	"errors"
	"os"
	"strings"

	"github.com/goforj/atlas/files"
)

// writeJSONServer merges one Atlas MCP server into an agent-owned JSON configuration file.
func writeJSONServer(path string, key string, server MCPServerConfig) error {
	settings, err := readJSONSettings(path)
	if errors.Is(err, os.ErrNotExist) {
		settings = map[string]any{}
		err = nil
	}
	if err != nil {
		return err
	}
	servers, ok := settings[key].(map[string]any)
	if !ok {
		servers = map[string]any{}
		settings[key] = servers
	}
	servers[server.Name] = map[string]any{
		"command": server.Command,
		"args":    server.Args(),
		"cwd":     server.CWD,
	}
	return writeJSONSettings(path, settings)
}

// removeJSONServer removes one Atlas MCP server without disturbing unrelated settings or servers.
func removeJSONServer(path string, key string, serverName string) error {
	settings, err := readJSONSettings(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	servers, ok := settings[key].(map[string]any)
	if !ok {
		return nil
	}
	delete(servers, serverName)
	if len(servers) == 0 {
		delete(settings, key)
	}
	if len(settings) == 0 {
		return os.Remove(path)
	}
	return writeJSONSettings(path, settings)
}

// readJSONSettings loads an existing agent configuration without inventing settings when it is absent.
func readJSONSettings(path string) (map[string]any, error) {
	settings := map[string]any{}
	content, err := os.ReadFile(path)
	if err != nil {
		return settings, err
	}
	if len(strings.TrimSpace(string(content))) == 0 {
		return settings, nil
	}
	if err := json.Unmarshal(content, &settings); err != nil {
		return nil, err
	}
	return settings, nil
}

// writeJSONSettings writes deterministic agent settings after a targeted merge.
func writeJSONSettings(path string, settings map[string]any) error {
	content, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return files.WriteFile(path, append(content, '\n'))
}

// replaceTOMLServer replaces Atlas's table while preserving unrelated TOML text and comments.
func replaceTOMLServer(existing string, server MCPServerConfig) string {
	base := removeTOMLServer(existing, server.Name)
	block := "[mcp_servers." + server.Name + "]\ncommand = " + quoteTOML(server.Command) + "\nargs = [" + quoteTOML(server.Args()[0]) + "]\ncwd = " + quoteTOML(server.CWD) + "\n"
	base = strings.TrimRight(base, "\n")
	if base == "" {
		return block
	}
	return base + "\n\n" + block
}

// removeTOMLServer removes Atlas's exact MCP table and leaves every other table untouched.
func removeTOMLServer(existing string, serverName string) string {
	target := "[mcp_servers." + serverName + "]"
	lines := strings.Split(existing, "\n")
	kept := make([]string, 0, len(lines))
	skipping := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == target {
			skipping = true
			continue
		}
		if skipping && strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			skipping = false
		}
		if !skipping {
			kept = append(kept, line)
		}
	}
	return strings.TrimSpace(strings.Join(kept, "\n"))
}

// quoteTOML quotes the small string surface used by Atlas MCP configuration.
func quoteTOML(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}
