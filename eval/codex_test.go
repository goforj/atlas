package eval

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	fakeCodexAdapterServerEnv = "ATLAS_FAKE_CODEX_ADAPTER_SERVER"
	fakeCodexAdapterFloodEnv  = "ATLAS_FAKE_CODEX_ADAPTER_FLOOD"
)

// TestAdapterRunsFreshAttributedDiagnosticSession exercises guidance attribution and event normalization through the real protocol client.
func TestAdapterRunsFreshAttributedDiagnosticSession(t *testing.T) {
	projectRoot := t.TempDir()
	homeRoot := t.TempDir()
	credential := filepath.Join(t.TempDir(), "auth.json")
	if err := os.WriteFile(credential, []byte(`{"token":"fixture"}`), 0o600); err != nil {
		t.Fatalf("write credential: %v", err)
	}
	adapter, err := NewCodexAgent(CodexOptions{
		Executable:       os.Args[0],
		Arguments:        []string{"-test.run=TestFakeCodexAdapterServer"},
		Model:            "gpt-test",
		ModelProvider:    "openai",
		CredentialSource: credential,
		Environment:      []string{fakeCodexAdapterServerEnv + "=1"},
	})
	if err != nil {
		t.Fatalf("NewCodexAgent(): %v", err)
	}
	capabilities, err := adapter.Capabilities(context.Background())
	if err != nil || len(capabilities.Capabilities) != 0 {
		t.Fatalf("Capabilities() = %#v, %v", capabilities, err)
	}
	prepared, err := adapter.Prepare(context.Background(), RunEnvironment{ProjectRoot: projectRoot, HomeRoot: homeRoot}, Guidance{
		Profile: "agents",
		Files:   map[string][]byte{"AGENTS.md": []byte("Use GoForj generators.\n")},
	})
	if err != nil {
		t.Fatalf("Prepare(): %v", err)
	}
	defer func() {
		if err := prepared.Close(context.Background()); err != nil {
			t.Errorf("close preparation: %v", err)
		}
	}()
	if prepared.Agent().ExecutableDigest == "" || prepared.Agent().Model != "gpt-test" {
		t.Fatalf("prepared agent = %#v", prepared.Agent())
	}
	if _, err := os.Stat(filepath.Join(homeRoot, "auth.json")); err != nil {
		t.Fatalf("private auth missing: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	session, err := adapter.Start(ctx, prepared.Agent())
	if err != nil {
		t.Fatalf("Start(): %v", err)
	}
	if identity := session.Identity(); identity.Version != "fake-codex/1" || identity.Model != "gpt-test" || identity.ModelProvider != "openai" {
		t.Fatalf("session identity = %#v", identity)
	}
	defer func() {
		if err := session.Close(context.Background()); err != nil {
			t.Errorf("close session: %v", err)
		}
	}()
	turn, err := session.Turn(ctx, AgentTurn{Prompt: "add an invoice route"})
	if err != nil || !turn.Accepted {
		t.Fatalf("Turn() = %#v, %v", turn, err)
	}
	result, err := session.Wait(ctx)
	if err != nil {
		t.Fatalf("Wait(): %v", err)
	}
	if result.Outcome != AgentCompleted || result.Message != "Implemented the route." {
		t.Fatalf("agent result = %#v", result)
	}
	for _, kind := range []EventKind{EventCommandStarted, EventCommandFinished, EventFileWrite, EventMessage, EventRunFinished} {
		if !containsEvent(result.Events, kind) {
			t.Fatalf("events missing %s: %#v", kind, result.Events)
		}
	}
	for _, event := range result.Events {
		if event.Kind == EventCommandStarted && event.Fields["evidence"] != "provider-telemetry" {
			t.Fatalf("command telemetry was presented as trusted evidence: %#v", event)
		}
		if event.Kind == EventFileWrite && event.Fields["kind"] != "update" {
			t.Fatalf("structured file-change kind was not normalized: %#v", event)
		}
	}
}

// TestAdapterRejectsUnexpectedInstructionSources prevents host guidance from contaminating a treatment.
func TestAdapterRejectsUnexpectedInstructionSources(t *testing.T) {
	projectRoot := t.TempDir()
	homeRoot := t.TempDir()
	credential := filepath.Join(t.TempDir(), "auth.json")
	if err := os.WriteFile(credential, []byte(`{}`), 0o600); err != nil {
		t.Fatalf("write credential: %v", err)
	}
	adapter, err := NewCodexAgent(CodexOptions{
		Executable:       os.Args[0],
		Arguments:        []string{"-test.run=TestFakeCodexAdapterServer"},
		Model:            "gpt-test",
		ModelProvider:    "openai",
		CredentialSource: credential,
		Environment:      []string{fakeCodexAdapterServerEnv + "=1", "ATLAS_FAKE_EXTRA_GUIDANCE=1"},
	})
	if err != nil {
		t.Fatalf("NewCodexAgent(): %v", err)
	}
	prepared, err := adapter.Prepare(context.Background(), RunEnvironment{ProjectRoot: projectRoot, HomeRoot: homeRoot}, Guidance{Profile: "none"})
	if err != nil {
		t.Fatalf("Prepare(): %v", err)
	}
	defer prepared.Close(context.Background())
	if _, err := adapter.Start(context.Background(), prepared.Agent()); err == nil || !strings.Contains(err.Error(), "instruction sources") {
		t.Fatalf("Start() error = %v, want instruction attribution failure", err)
	}
}

// TestAdapterPrepareRollsBackGuidanceOnCredentialFailure prevents a failed treatment setup from contaminating a later attempt.
func TestAdapterPrepareRollsBackGuidanceOnCredentialFailure(t *testing.T) {
	projectRoot := t.TempDir()
	adapter, err := NewCodexAgent(CodexOptions{Executable: os.Args[0], Model: "gpt-test", CredentialSource: filepath.Join(t.TempDir(), "missing-auth.json")})
	if err != nil {
		t.Fatalf("NewCodexAgent(): %v", err)
	}
	_, err = adapter.Prepare(context.Background(), RunEnvironment{ProjectRoot: projectRoot, HomeRoot: t.TempDir()}, Guidance{
		Files: map[string][]byte{".agent/AGENTS.md": []byte("fixture")},
	})
	if err == nil {
		t.Fatal("Prepare() succeeded without a credential")
	}
	if _, statErr := os.Stat(filepath.Join(projectRoot, ".agent")); !os.IsNotExist(statErr) {
		t.Fatalf("failed preparation retained guidance state: %v", statErr)
	}
}

// TestAdapterFailsExplicitlyWhenTelemetryOverflows prevents a dropped terminal notification from masquerading as a stalled turn.
func TestAdapterFailsExplicitlyWhenTelemetryOverflows(t *testing.T) {
	t.Setenv(fakeCodexAdapterFloodEnv, "1")
	projectRoot := t.TempDir()
	homeRoot := t.TempDir()
	credential := filepath.Join(t.TempDir(), "auth.json")
	if err := os.WriteFile(credential, []byte(`{}`), 0o600); err != nil {
		t.Fatalf("write credential: %v", err)
	}
	adapter, err := NewCodexAgent(CodexOptions{
		Executable:       os.Args[0],
		Arguments:        []string{"-test.run=TestFakeCodexAdapterServer"},
		Model:            "gpt-test",
		CredentialSource: credential,
		Environment: []string{
			fakeCodexAdapterServerEnv + "=1",
			fakeCodexAdapterFloodEnv + "=1",
		},
	})
	if err != nil {
		t.Fatalf("NewCodexAgent(): %v", err)
	}
	prepared, err := adapter.Prepare(context.Background(), RunEnvironment{ProjectRoot: projectRoot, HomeRoot: homeRoot}, Guidance{Profile: "none"})
	if err != nil {
		t.Fatalf("Prepare(): %v", err)
	}
	defer prepared.Close(context.Background())
	session, err := adapter.Start(context.Background(), prepared.Agent())
	if err != nil {
		t.Fatalf("Start(): %v", err)
	}
	defer session.Close(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := session.Turn(ctx, AgentTurn{Prompt: "emit telemetry"}); err != nil {
		t.Fatalf("Turn(): %v", err)
	}
	if _, err := session.Wait(ctx); err == nil || !strings.Contains(err.Error(), "telemetry overflow") {
		t.Fatalf("Wait() error = %v, want telemetry overflow", err)
	}
}

// TestFakeCodexAdapterServer provides deterministic app-server notifications for adapter tests.
func TestFakeCodexAdapterServer(t *testing.T) {
	if os.Getenv(fakeCodexAdapterServerEnv) != "1" {
		return
	}
	runFakeCodexAdapterServer()
	os.Exit(0)
}

// runFakeCodexAdapterServer implements only the app-server subset the adapter consumes.
func runFakeCodexAdapterServer() {
	scanner := bufio.NewScanner(os.Stdin)
	var projectRoot string
	for scanner.Scan() {
		var request struct {
			ID     *uint64         `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if json.Unmarshal(scanner.Bytes(), &request) != nil || request.ID == nil {
			continue
		}
		switch request.Method {
		case "initialize":
			writeFakeAdapterResponse(*request.ID, map[string]any{"userAgent": "fake-codex/1"})
		case "thread/start":
			var params struct {
				CWD            string `json:"cwd"`
				ApprovalPolicy string `json:"approvalPolicy"`
				Sandbox        string `json:"sandbox"`
			}
			_ = json.Unmarshal(request.Params, &params)
			projectRoot = params.CWD
			sources := []string{}
			if _, err := os.Stat(filepath.Join(projectRoot, "AGENTS.md")); err == nil {
				sources = append(sources, filepath.Join(projectRoot, "AGENTS.md"))
			}
			if os.Getenv("ATLAS_FAKE_EXTRA_GUIDANCE") == "1" {
				sources = append(sources, filepath.Join(os.Getenv("HOME"), "AGENTS.md"))
			}
			writeFakeAdapterResponse(*request.ID, map[string]any{
				"model": "gpt-test", "modelProvider": "openai", "approvalPolicy": params.ApprovalPolicy, "sandbox": map[string]any{"type": "dangerFullAccess"}, "instructionSources": sources,
				"thread": map[string]any{
					"id":        "thread-1",
					"ephemeral": true,
				},
			})
		case "turn/start":
			if os.Getenv(fakeCodexAdapterFloodEnv) == "1" {
				for range 1025 {
					writeFakeAdapterNotification("server/flood", map[string]any{})
				}
			}
			writeFakeAdapterResponse(*request.ID, map[string]any{"turn": map[string]any{"id": "turn-1"}})
			writeFakeAdapterNotification("item/started", fakeAdapterItemParams(map[string]any{
				"id": "command-1", "type": "commandExecution", "command": "forj make:controller invoices", "commandActions": []any{}, "cwd": projectRoot, "status": "inProgress",
			}))
			writeFakeAdapterNotification("item/completed", fakeAdapterItemParams(map[string]any{
				"id": "command-1", "type": "commandExecution", "command": "forj make:controller invoices", "commandActions": []any{}, "cwd": projectRoot, "status": "completed", "exitCode": 0,
			}))
			writeFakeAdapterNotification("item/completed", fakeAdapterItemParams(map[string]any{
				"id": "change-1", "type": "fileChange", "status": "completed", "changes": []map[string]any{{"path": "internal/invoices/controller.go", "kind": map[string]string{"type": "update"}, "diff": ""}},
			}))
			writeFakeAdapterNotification("item/completed", fakeAdapterItemParams(map[string]any{
				"id": "message-1", "type": "agentMessage", "text": "Implemented the route.",
			}))
			writeFakeAdapterNotification("turn/completed", map[string]any{
				"threadId": "thread-1", "turn": map[string]any{"id": "turn-1", "status": "completed", "items": []any{}},
			})
		case "turn/interrupt":
			writeFakeAdapterResponse(*request.ID, map[string]any{})
		default:
			writeFakeAdapterResponse(*request.ID, map[string]any{})
		}
	}
}

// fakeAdapterItemParams attaches the stable thread and turn identity to one item.
func fakeAdapterItemParams(item map[string]any) map[string]any {
	return map[string]any{"threadId": "thread-1", "turnId": "turn-1", "item": item}
}

// writeFakeAdapterResponse emits one JSON-RPC response.
func writeFakeAdapterResponse(id uint64, result any) {
	writeFakeAdapterMessage(map[string]any{"id": id, "result": result})
}

// writeFakeAdapterNotification emits one JSON-RPC notification.
func writeFakeAdapterNotification(method string, params any) {
	writeFakeAdapterMessage(map[string]any{"method": method, "params": params})
}

// writeFakeAdapterMessage serializes protocol fixtures without test-runner prefixes.
func writeFakeAdapterMessage(message map[string]any) {
	encoded, _ := json.Marshal(message)
	_, _ = fmt.Fprintln(os.Stdout, string(encoded))
}

// containsEvent reports whether normalized telemetry includes one lifecycle kind.
func containsEvent(events []Event, kind EventKind) bool {
	for _, event := range events {
		if event.Kind == kind {
			return true
		}
	}
	return false
}
