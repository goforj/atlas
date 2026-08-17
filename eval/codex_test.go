package eval

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

const (
	fakeCodexAdapterServerEnv = "ATLAS_FAKE_CODEX_ADAPTER_SERVER"
	fakeCodexAdapterFloodEnv  = "ATLAS_FAKE_CODEX_ADAPTER_FLOOD"
	fakeCodexAdapterPacedEnv  = "ATLAS_FAKE_CODEX_ADAPTER_PACED"
)

// fakeAdapterThreadStartResult mirrors the stable thread identity returned by Codex.
type fakeAdapterThreadStartResult struct {
	Model              string                   `json:"model"`
	ModelProvider      string                   `json:"modelProvider"`
	ApprovalPolicy     string                   `json:"approvalPolicy"`
	Sandbox            fakeAdapterSandboxResult `json:"sandbox"`
	InstructionSources []string                 `json:"instructionSources"`
	Thread             fakeAdapterThreadResult  `json:"thread"`
}

// fakeAdapterSandboxResult mirrors Codex's typed sandbox response.
type fakeAdapterSandboxResult struct {
	Type string `json:"type"`
}

// fakeAdapterThreadResult identifies one fake Codex thread.
type fakeAdapterThreadResult struct {
	ID        string `json:"id"`
	Ephemeral bool   `json:"ephemeral"`
}

// fakeAdapterTurnStartResult identifies one fake Codex turn.
type fakeAdapterTurnStartResult struct {
	Turn fakeAdapterTurnResult `json:"turn"`
}

// fakeAdapterTurnResult contains the stable turn identity used by the adapter.
type fakeAdapterTurnResult struct {
	ID string `json:"id"`
}

// TestAdapterRunsFreshAttributedDiagnosticSession exercises guidance attribution and event normalization through the real protocol client.
func TestAdapterRunsFreshAttributedDiagnosticSession(t *testing.T) {
	projectRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectRoot, "AGENTS.md"), []byte("Use GoForj generators.\n"), 0o644); err != nil {
		t.Fatalf("write project guidance: %v", err)
	}
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
	capabilities, err := adapter.Properties(context.Background())
	if err != nil || !reflect.DeepEqual(capabilities.Properties, []Capability{CapabilityFinalResponseCapture}) {
		t.Fatalf("Properties() = %#v, %v", capabilities, err)
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
	if identity := session.Identity(); identity.Version != "fake-codex/1" || identity.Model != "gpt-test" || identity.ModelProvider != "openai" || !strings.HasPrefix(identity.SessionDigest, "sha256:") {
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

// TestCodexAgentStartRejectsLauncherDigestDrift keeps preparation identity from being reused after the selected launcher changes.
func TestCodexAgentStartRejectsLauncherDigestDrift(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink replacement is not portable on Windows test hosts")
	}
	launcher := filepath.Join(t.TempDir(), "codex")
	if err := os.Symlink(os.Args[0], launcher); err != nil {
		t.Fatalf("create launcher symlink: %v", err)
	}
	credential := filepath.Join(t.TempDir(), "auth.json")
	if err := os.WriteFile(credential, []byte(`{}`), 0o600); err != nil {
		t.Fatalf("write credential: %v", err)
	}
	adapter, err := NewCodexAgent(CodexOptions{Executable: launcher, Model: "gpt-test", CredentialSource: credential})
	if err != nil {
		t.Fatalf("NewCodexAgent(): %v", err)
	}
	prepared, err := adapter.Prepare(context.Background(), RunEnvironment{ProjectRoot: t.TempDir(), HomeRoot: t.TempDir()}, Guidance{Profile: "none"})
	if err != nil {
		t.Fatalf("Prepare(): %v", err)
	}
	defer prepared.Close(context.Background())
	if err := os.Remove(launcher); err != nil {
		t.Fatalf("remove launcher symlink: %v", err)
	}
	if err := os.WriteFile(launcher, []byte("replacement"), 0o700); err != nil {
		t.Fatalf("replace launcher: %v", err)
	}
	if _, err := adapter.Start(context.Background(), prepared.Agent()); err == nil || !strings.Contains(err.Error(), "pre-exec launcher digest") {
		t.Fatalf("Start() error = %v, want launcher integrity failure", err)
	}
}

// TestCodexAgentStartRejectsLauncherPathDrift keeps a later PATH resolution from selecting a different prepared launcher.
func TestCodexAgentStartRejectsLauncherPathDrift(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink replacement is not portable on Windows test hosts")
	}
	firstDirectory := t.TempDir()
	secondDirectory := t.TempDir()
	first := filepath.Join(firstDirectory, "codex")
	second := filepath.Join(secondDirectory, "codex")
	for _, launcher := range []string{first, second} {
		if err := os.Symlink(os.Args[0], launcher); err != nil {
			t.Fatalf("create launcher symlink: %v", err)
		}
	}
	credential := filepath.Join(t.TempDir(), "auth.json")
	if err := os.WriteFile(credential, []byte(`{}`), 0o600); err != nil {
		t.Fatalf("write credential: %v", err)
	}
	t.Setenv("PATH", firstDirectory)
	adapter, err := NewCodexAgent(CodexOptions{Executable: "codex", Model: "gpt-test", CredentialSource: credential})
	if err != nil {
		t.Fatalf("NewCodexAgent(): %v", err)
	}
	prepared, err := adapter.Prepare(context.Background(), RunEnvironment{ProjectRoot: t.TempDir(), HomeRoot: t.TempDir()}, Guidance{Profile: "none"})
	if err != nil {
		t.Fatalf("Prepare(): %v", err)
	}
	defer prepared.Close(context.Background())
	t.Setenv("PATH", secondDirectory)
	if _, err := adapter.Start(context.Background(), prepared.Agent()); err == nil || !strings.Contains(err.Error(), "pre-exec launcher digest") {
		t.Fatalf("Start() error = %v, want launcher integrity failure", err)
	}
}

// TestAdapterBoundsPacedTelemetryByCount verifies a slow producer cannot evade aggregate retention limits.
func TestAdapterBoundsPacedTelemetryByCount(t *testing.T) {
	session := startTelemetryTestSession(t, "count")
	defer session.Close(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := session.Turn(ctx, AgentTurn{Prompt: "emit paced telemetry"}); err != nil {
		t.Fatal(err)
	}
	result, err := session.Wait(ctx)
	if err == nil || !strings.Contains(err.Error(), "telemetry overflow") {
		t.Fatalf("Wait() = %#v, %v, want aggregate overflow", result, err)
	}
	if result.Telemetry == nil || result.Telemetry.EventsDropped != 1 || result.Telemetry.NotificationsDropped != 0 || len(result.Events) != defaultProviderEventLimit {
		t.Fatalf("paced count telemetry = %#v with %d events", result.Telemetry, len(result.Events))
	}
	if result.Telemetry.CommandsObserved != defaultProviderEventLimit+1 {
		t.Fatalf("commands observed = %d, want %d", result.Telemetry.CommandsObserved, defaultProviderEventLimit+1)
	}
}

// TestAdapterBoundsPacedTelemetryByBytes verifies individually valid messages cannot accumulate beyond the retained byte budget.
func TestAdapterBoundsPacedTelemetryByBytes(t *testing.T) {
	session := startTelemetryTestSession(t, "bytes")
	defer session.Close(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := session.Turn(ctx, AgentTurn{Prompt: "emit large paced telemetry"}); err != nil {
		t.Fatal(err)
	}
	result, err := session.Wait(ctx)
	if err == nil || !strings.Contains(err.Error(), "telemetry overflow") {
		t.Fatalf("Wait() = telemetry %#v, %v, want aggregate overflow", result.Telemetry, err)
	}
	if result.Telemetry == nil || result.Telemetry.EventsDropped != 1 || result.Telemetry.BytesDropped == 0 || result.Telemetry.NotificationsDropped != 0 || len(result.Events) != 1 {
		t.Fatalf("paced byte telemetry = %#v with %d events", result.Telemetry, len(result.Events))
	}
}

// startTelemetryTestSession starts the protocol fixture with one paced stream mode.
func startTelemetryTestSession(t *testing.T, mode string) EvaluationSession {
	t.Helper()
	credential, err := NewCodexCredential([]byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	adapter, err := NewCodexAgent(CodexOptions{
		Executable:  os.Args[0],
		Arguments:   []string{"-test.run=TestFakeCodexAdapterServer"},
		Model:       "gpt-test",
		Credential:  credential,
		Environment: []string{fakeCodexAdapterServerEnv + "=1", fakeCodexAdapterPacedEnv + "=" + mode},
	})
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := adapter.Prepare(context.Background(), RunEnvironment{ProjectRoot: t.TempDir(), HomeRoot: t.TempDir()}, Guidance{Profile: "none"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = prepared.Close(context.Background()) })
	session, err := adapter.Start(context.Background(), prepared.Agent())
	if err != nil {
		t.Fatal(err)
	}
	return session
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

// TestAdapterPrepareDoesNotMutateGuidanceOnCredentialFailure keeps native Project treatment ownership outside the provider adapter.
func TestAdapterPrepareDoesNotMutateGuidanceOnCredentialFailure(t *testing.T) {
	projectRoot := t.TempDir()
	_, err := NewCodexAgent(CodexOptions{Executable: os.Args[0], Model: "gpt-test", CredentialSource: filepath.Join(t.TempDir(), "missing-auth.json")})
	if err == nil {
		t.Fatal("NewCodexAgent() succeeded without a credential")
	}
	if _, statErr := os.Stat(filepath.Join(projectRoot, ".agent")); !os.IsNotExist(statErr) {
		t.Fatalf("provider adapter mutated project guidance: %v", statErr)
	}
}

// TestAdapterFreezesCredentialBeforeTreatments prevents a replaced source path from changing paired provider authority.
func TestAdapterFreezesCredentialBeforeTreatments(t *testing.T) {
	source := filepath.Join(t.TempDir(), "auth.json")
	original := []byte(`{"access_token":"first-treatment-secret"}`)
	replacement := []byte(`{"access_token":"replacement-secret"}`)
	if err := os.WriteFile(source, original, 0o600); err != nil {
		t.Fatal(err)
	}
	adapter, err := NewCodexAgent(CodexOptions{Executable: os.Args[0], Model: "gpt-test", CredentialSource: source})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, replacement, 0o600); err != nil {
		t.Fatal(err)
	}
	var authorityDigest string
	for range 2 {
		homeRoot := t.TempDir()
		prepared, err := adapter.Prepare(context.Background(), RunEnvironment{ProjectRoot: t.TempDir(), HomeRoot: homeRoot}, Guidance{})
		if err != nil {
			t.Fatal(err)
		}
		body, err := os.ReadFile(filepath.Join(homeRoot, "auth.json"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(body, original) || bytes.Contains(body, replacement) {
			t.Fatalf("materialized credential = %q, want frozen authority", body)
		}
		if authorityDigest == "" {
			authorityDigest = prepared.Agent().AuthorityDigest
		} else if prepared.Agent().AuthorityDigest != authorityDigest {
			t.Fatalf("paired authority digests differ: %q and %q", authorityDigest, prepared.Agent().AuthorityDigest)
		}
		if err := prepared.Close(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	redacted := adapter.credential.Redactor(NewRedactor(nil)).Text(string(original) + " " + string(replacement))
	if strings.Contains(redacted, "first-treatment-secret") || !strings.Contains(redacted, "replacement-secret") {
		t.Fatalf("frozen redaction did not match frozen authority: %q", redacted)
	}
}

// TestCodexCredentialRedactsEveryStringLeaf keeps provider-extension authority out of every redacted output shape.
func TestCodexCredentialRedactsEveryStringLeaf(t *testing.T) {
	credential, err := NewCodexCredential([]byte(`{"provider":"custom-provider","oauth_token":"extension-secret","nested":{"cookie":"session-secret","chain":["array-secret",{"token":"deep-secret"}]}}`))
	if err != nil {
		t.Fatal(err)
	}
	redactor := credential.Redactor(NewRedactor(nil))
	for _, value := range []string{"custom-provider", "extension-secret", "session-secret", "array-secret", "deep-secret"} {
		if redacted := redactor.Text("bare " + value); strings.Contains(redacted, value) {
			t.Fatalf("credential leaf %q survived text redaction: %q", value, redacted)
		}
		if redacted := redactor.Event(Event{Fields: map[string]string{"value": value}}); strings.Contains(redacted.Fields["value"], value) {
			t.Fatalf("credential leaf %q survived event redaction: %#v", value, redacted)
		}
		redactedJSON := redactor.JSONValue(map[string]any{"value": value}).(map[string]any)
		if strings.Contains(redactedJSON["value"].(string), value) {
			t.Fatalf("credential leaf %q survived JSON redaction: %#v", value, redactedJSON)
		}
	}
}

// TestCodexCredentialRedactsTopLevelAndArrayStrings keeps the schema-independent walker complete for every JSON value shape.
func TestCodexCredentialRedactsTopLevelAndArrayStrings(t *testing.T) {
	for _, test := range []struct {
		body   string
		secret string
	}{
		{body: `"top-level-secret"`, secret: "top-level-secret"},
		{body: `["array-secret",{"nested":["deep-secret"]}]`, secret: "array-secret"},
		{body: `["array-secret",{"nested":["deep-secret"]}]`, secret: "deep-secret"},
	} {
		credential, err := NewCodexCredential([]byte(test.body))
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Contains(credential.redactionSecrets, test.secret) {
			t.Fatalf("credential secrets = %q, want %q", credential.redactionSecrets, test.secret)
		}
		if redacted := credential.Redactor(NewRedactor(nil)).Text("value=" + test.secret); strings.Contains(redacted, test.secret) {
			t.Fatalf("credential leaf %q survived redaction: %q", test.secret, redacted)
		}
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
			writeFakeAdapterResponse(*request.ID, fakeAdapterThreadStartResult{
				Model:              "gpt-test",
				ModelProvider:      "openai",
				ApprovalPolicy:     params.ApprovalPolicy,
				Sandbox:            fakeAdapterSandboxResult{Type: "dangerFullAccess"},
				InstructionSources: sources,
				Thread: fakeAdapterThreadResult{
					ID:        "thread-1",
					Ephemeral: true,
				},
			})
		case "turn/start":
			pacedMode := os.Getenv(fakeCodexAdapterPacedEnv)
			if pacedMode != "" {
				writeFakeAdapterResponse(*request.ID, fakeAdapterTurnStartResult{Turn: fakeAdapterTurnResult{ID: "turn-1"}})
				emitPacedAdapterTelemetry(pacedMode)
				continue
			}
			if os.Getenv(fakeCodexAdapterFloodEnv) == "1" {
				for range 1025 {
					writeFakeAdapterNotification("server/flood", map[string]any{})
				}
			}
			writeFakeAdapterResponse(*request.ID, fakeAdapterTurnStartResult{Turn: fakeAdapterTurnResult{ID: "turn-1"}})
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

// emitPacedAdapterTelemetry keeps queue occupancy low while exceeding aggregate normalized retention.
func emitPacedAdapterTelemetry(mode string) {
	switch mode {
	case "count":
		for index := range defaultProviderEventLimit + 1 {
			writeFakeAdapterNotification("item/started", fakeAdapterItemParams(map[string]any{
				"id": fmt.Sprintf("command-%d", index), "type": "commandExecution", "command": "true", "cwd": "/project",
			}))
			time.Sleep(time.Millisecond)
		}
	case "bytes":
		text := strings.Repeat("x", 5<<20)
		for index := range 2 {
			writeFakeAdapterNotification("item/completed", fakeAdapterItemParams(map[string]any{
				"id": fmt.Sprintf("message-%d", index), "type": "agentMessage", "text": text,
			}))
			time.Sleep(20 * time.Millisecond)
		}
	}
	writeFakeAdapterNotification("turn/completed", map[string]any{
		"threadId": "thread-1", "turn": map[string]any{"id": "turn-1", "status": "completed"},
	})
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
