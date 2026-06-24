package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestStdioProtocolToolDiscoveryAndCriticalCalls(t *testing.T) {
	client := startStdioProtocolServer(t)
	defer client.close(t)

	client.request(t, "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"clientInfo": map[string]any{
			"name":    "atlas-protocol-test",
			"version": "1.0.0",
		},
	})
	client.notify(t, "notifications/initialized", nil)

	list := client.request(t, "tools/list", map[string]any{})
	for _, name := range stableToolNames() {
		if !strings.Contains(jsonString(t, list), `"name":"`+name+`"`) {
			t.Fatalf("tools/list missing %s:\n%s", name, jsonString(t, list))
		}
	}

	calls := []struct {
		name      string
		arguments map[string]any
		want      string
	}{
		{name: "workflow-plan", arguments: map[string]any{"task": "add checkout route", "app": "marketplace"}, want: "goforj-add-http-route"},
		{name: "docs-section-pack", arguments: map[string]any{"workflow_id": "goforj-add-http-route"}, want: "applications/routes.md"},
		{name: "resource-inventory", arguments: map[string]any{}, want: "frontend_kit"},
		{name: "validation-plan", arguments: map[string]any{"task": "add checkout route", "app": "marketplace"}, want: "forj marketplace build"},
	}
	for _, call := range calls {
		response := client.request(t, "tools/call", map[string]any{
			"name":      call.name,
			"arguments": call.arguments,
		})
		text := jsonString(t, response)
		if !strings.Contains(text, call.want) {
			t.Fatalf("%s response missing %q:\n%s", call.name, call.want, text)
		}
		if strings.Contains(text, `"error"`) {
			t.Fatalf("%s returned transport error:\n%s", call.name, text)
		}
	}

	malformed := client.request(t, "tools/call", map[string]any{
		"name":      "workflow-plan",
		"arguments": map[string]any{},
	})
	text := jsonString(t, malformed)
	if strings.Contains(text, `"error"`) || !strings.Contains(text, "task") {
		t.Fatalf("malformed tool call should return bounded tool error, got:\n%s", text)
	}
}

func TestMCPStdioHelperProcess(t *testing.T) {
	if os.Getenv("ATLAS_MCP_TEST_SERVER") != "1" {
		return
	}
	if err := ServeStdio(fixtureServer()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	os.Exit(0)
}

type stdioProtocolClient struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Scanner
	stderr *bytes.Buffer
	nextID int
}

func startStdioProtocolServer(t *testing.T) *stdioProtocolClient {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestMCPStdioHelperProcess$")
	cmd.Env = append(os.Environ(), "ATLAS_MCP_TEST_SERVER=1")
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start helper: %v\nstderr:\n%s", err, stderr.String())
	}
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	return &stdioProtocolClient{cmd: cmd, stdin: stdin, stdout: scanner, stderr: stderr, nextID: 1}
}

func (c *stdioProtocolClient) request(t *testing.T, method string, params map[string]any) map[string]any {
	t.Helper()
	id := c.nextID
	c.nextID++
	message := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
	}
	if params != nil {
		message["params"] = params
	}
	c.write(t, message)
	if !c.stdout.Scan() {
		t.Fatalf("read response for %s: %v\nstderr:\n%s", method, c.stdout.Err(), c.stderr.String())
	}
	var response map[string]any
	line := c.stdout.Bytes()
	if err := json.Unmarshal(line, &response); err != nil {
		t.Fatalf("stdout must be MCP JSON only; got %q: %v\nstderr:\n%s", string(line), err, c.stderr.String())
	}
	if got := response["id"]; got != float64(id) {
		t.Fatalf("response id = %v, want %d: %#v", got, id, response)
	}
	return response
}

func (c *stdioProtocolClient) notify(t *testing.T, method string, params map[string]any) {
	t.Helper()
	message := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
	}
	if params != nil {
		message["params"] = params
	}
	c.write(t, message)
}

func (c *stdioProtocolClient) write(t *testing.T, message map[string]any) {
	t.Helper()
	content, err := json.Marshal(message)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	if _, err := c.stdin.Write(append(content, '\n')); err != nil {
		t.Fatalf("write request: %v\nstderr:\n%s", err, c.stderr.String())
	}
}

func (c *stdioProtocolClient) close(t *testing.T) {
	t.Helper()
	if c.stdin != nil {
		_ = c.stdin.Close()
	}
	if err := c.cmd.Wait(); err != nil {
		t.Fatalf("helper exited: %v\nstderr:\n%s", err, c.stderr.String())
	}
}

func stableToolNames() []string {
	return []string{
		"application-info",
		"project-layout",
		"search-docs",
		"read-doc-section",
		"read-doc-neighborhood",
		"list-doc-headings",
		"explain-api",
		"route-list",
		"schedule-list",
		"command-list",
		"database-connections",
		"database-schema",
		"database-query",
		"read-log-entries",
		"last-error",
		"get-absolute-url",
		"browser-logs",
		"metrics-metadata",
		"runtime-snapshot",
		"debug-plan",
		"workflow-plan",
		"registration-points",
		"validation-plan",
		"wire-diagnostics",
		"scenario-guide",
		"resource-inventory",
		"generated-file-policy",
		"command-advice",
		"docs-section-pack",
		"version-alignment",
		"workflow-scorecard",
	}
}

func jsonString(t *testing.T, value any) string {
	t.Helper()
	content, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	return string(content)
}
