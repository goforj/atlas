package codexappserver

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

const (
	fakeAppServerEnv      = "ATLAS_FAKE_CODEX_APP_SERVER"
	fakeFloodEnv          = "ATLAS_FAKE_CODEX_FLOOD_NOTIFICATIONS"
	fakePolicyMismatchEnv = "ATLAS_FAKE_CODEX_POLICY_MISMATCH"
)

// TestClientAttributesFreshThreads verifies the adapter records effective identity and instruction provenance from Codex.
func TestClientAttributesFreshThreads(t *testing.T) {
	client := startFakeClient(t)
	defer closeClient(t, client)
	if client.UserAgent() != "fake-codex/1" {
		t.Fatalf("user agent = %q", client.UserAgent())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	first, err := client.StartThread(ctx, fakeThreadOptions(t.TempDir()))
	if err != nil {
		t.Fatalf("start first thread: %v", err)
	}
	second, err := client.StartThread(ctx, fakeThreadOptions(t.TempDir()))
	if err != nil {
		t.Fatalf("start second thread: %v", err)
	}
	if first.ID == second.ID {
		t.Fatalf("fresh threads reused id %q", first.ID)
	}
	if first.Model != "gpt-test" || first.ModelProvider != "openai" {
		t.Fatalf("effective identity = %s/%s", first.ModelProvider, first.Model)
	}
	if !first.Ephemeral {
		t.Fatal("thread was not ephemeral")
	}
	if first.ApprovalPolicy != "never" || first.Sandbox != "read-only" {
		t.Fatalf("effective policy = approvalPolicy %q, sandbox %q", first.ApprovalPolicy, first.Sandbox)
	}
	if got := strings.Join(first.InstructionSources, ","); got != "/project/AGENTS.md" {
		t.Fatalf("instruction sources = %q", got)
	}
}

// TestClientStartsAndInterruptsTurn verifies request correlation across lifecycle notifications.
func TestClientStartsAndInterruptsTurn(t *testing.T) {
	client := startFakeClient(t)
	defer closeClient(t, client)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	thread, err := client.StartThread(ctx, fakeThreadOptions(t.TempDir()))
	if err != nil {
		t.Fatalf("start thread: %v", err)
	}
	turn, err := client.StartTurn(ctx, thread.ID, "add an invoice route")
	if err != nil {
		t.Fatalf("start turn: %v", err)
	}
	if turn.ID == "" {
		t.Fatal("turn id is empty")
	}
	if err := client.InterruptTurn(ctx, thread.ID, turn.ID); err != nil {
		t.Fatalf("interrupt turn: %v", err)
	}
}

// TestClientDoesNotBlockResponsesBehindTelemetry proves notifications cannot prevent a request response from reaching its waiter.
func TestClientDoesNotBlockResponsesBehindTelemetry(t *testing.T) {
	t.Setenv(fakeFloodEnv, "1")
	client := startFakeClient(t)
	defer closeClient(t, client)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	thread, err := client.StartThread(ctx, fakeThreadOptions(t.TempDir()))
	if err != nil {
		t.Fatalf("start thread: %v", err)
	}
	if _, err := client.StartTurn(ctx, thread.ID, "emit telemetry"); err != nil {
		t.Fatalf("start turn after telemetry flood: %v", err)
	}
	if client.NotificationsDropped() == 0 {
		t.Fatal("telemetry overflow was not recorded")
	}
}

// TestClientDropsNotificationsBeyondByteLimit verifies large telemetry cannot consume unbounded memory.
func TestClientDropsNotificationsBeyondByteLimit(t *testing.T) {
	client := newNotificationTestClient(10)
	go client.deliverNotifications()

	client.deliverNotification(Notification{Method: "item", Params: json.RawMessage("1")})
	client.deliverNotification(Notification{Method: "item", Params: json.RawMessage("2")})
	client.deliverNotification(Notification{Method: "item", Params: json.RawMessage("23")})
	if got := client.NotificationsDropped(); got != 1 {
		t.Fatalf("notifications dropped = %d, want 1", got)
	}

	for _, want := range []string{"1", "2"} {
		notification := <-client.Notifications()
		if got := string(notification.Params); got != want {
			t.Fatalf("notification params = %q, want %q", got, want)
		}
	}
	client.closeNotifications()
	if _, ok := <-client.Notifications(); ok {
		t.Fatal("notifications channel remained open after queued notifications drained")
	}
}

// TestLockedBufferRetainsTail verifies stderr diagnostics remain bounded while preserving the most recent output.
func TestLockedBufferRetainsTail(t *testing.T) {
	buffer := lockedBuffer{limit: 5}
	if _, err := buffer.Write([]byte("abc")); err != nil {
		t.Fatalf("write initial stderr: %v", err)
	}
	if _, err := buffer.Write([]byte("defgh")); err != nil {
		t.Fatalf("write overflowing stderr: %v", err)
	}
	if got := buffer.String(); got != "defgh" {
		t.Fatalf("stderr = %q, want tail %q", got, "defgh")
	}
	if got := buffer.Dropped(); got != 3 {
		t.Fatalf("dropped stderr bytes = %d, want 3", got)
	}
	if _, err := buffer.Write([]byte("ij")); err != nil {
		t.Fatalf("write final stderr: %v", err)
	}
	if got := buffer.String(); got != "fghij" {
		t.Fatalf("stderr = %q, want tail %q", got, "fghij")
	}
	if got := buffer.Dropped(); got != 5 {
		t.Fatalf("dropped stderr bytes = %d, want 5", got)
	}
}

// TestClientRejectsMismatchedEffectivePolicy keeps a diagnostic session from silently weakening its requested policy.
func TestClientRejectsMismatchedEffectivePolicy(t *testing.T) {
	t.Setenv(fakePolicyMismatchEnv, "1")
	client := startFakeClient(t)
	defer closeClient(t, client)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := client.StartThread(ctx, fakeThreadOptions(t.TempDir()))
	if err == nil || !strings.Contains(err.Error(), "effective policy") {
		t.Fatalf("StartThread() error = %v, want effective policy mismatch", err)
	}
}

// TestClientCloseIsIdempotent keeps deferred and explicit cleanup from signaling a reused process identifier.
func TestClientCloseIsIdempotent(t *testing.T) {
	client := startFakeClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Close(ctx); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := client.Close(ctx); err != nil {
		t.Fatalf("second close: %v", err)
	}
}

// TestClientCloseCancelsBlockedNotificationDelivery verifies cleanup does not require an abandoned notifications receiver to drain telemetry.
func TestClientCloseCancelsBlockedNotificationDelivery(t *testing.T) {
	client := startFakeClient(t)
	client.deliverNotification(Notification{Method: "item", Params: json.RawMessage("1")})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Close(ctx); err != nil {
		t.Fatalf("close app-server: %v", err)
	}
	select {
	case _, ok := <-client.Notifications():
		if ok {
			t.Fatal("notifications channel delivered telemetry after client close")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("notification delivery remained blocked after client close")
	}
}

// TestFakeAppServerProcess implements the versioned protocol subset without a live model.
func TestFakeAppServerProcess(t *testing.T) {
	if os.Getenv(fakeAppServerEnv) != "1" {
		return
	}
	runFakeAppServer()
	os.Exit(0)
}

// startFakeClient launches the current test binary as an isolated protocol fixture.
func startFakeClient(t *testing.T) *Client {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	client, err := Start(ctx, StartOptions{
		Executable: os.Args[0],
		Arguments:  []string{"-test.run=TestFakeAppServerProcess"},
		Dir:        t.TempDir(),
		Env: []string{
			fakeAppServerEnv + "=1",
			fakeFloodEnv + "=" + os.Getenv(fakeFloodEnv),
			fakePolicyMismatchEnv + "=" + os.Getenv(fakePolicyMismatchEnv),
		},
	})
	if err != nil {
		t.Fatalf("start fake app-server: %v", err)
	}
	return client
}

// closeClient gives cleanup an independent budget so a failed test context cannot leak the fixture.
func closeClient(t *testing.T, client *Client) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Close(ctx); err != nil {
		t.Errorf("close app-server: %v", err)
	}
}

// newNotificationTestClient creates an in-memory telemetry client with a small aggregate byte limit.
func newNotificationTestClient(byteLimit int) *Client {
	return &Client{
		notifications:         make(chan Notification),
		notificationByteLimit: byteLimit,
		notificationWake:      make(chan struct{}, 1),
		notificationCancel:    make(chan struct{}),
	}
}

// fakeThreadOptions keeps attribution assertions identical across fresh-thread requests.
func fakeThreadOptions(root string) ThreadOptions {
	return ThreadOptions{
		Model:          "gpt-test",
		ModelProvider:  "openai",
		Dir:            root,
		ApprovalPolicy: "never",
		Sandbox:        "read-only",
		ServiceName:    "atlas_test",
		Ephemeral:      true,
	}
}

// runFakeAppServer responds to the protocol methods required by deterministic client tests.
func runFakeAppServer() {
	scanner := bufio.NewScanner(os.Stdin)
	threadNumber := 0
	turnNumber := 0
	for scanner.Scan() {
		var request struct {
			ID     *uint64         `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if json.Unmarshal(scanner.Bytes(), &request) != nil || request.ID == nil {
			continue
		}
		var result any = map[string]any{}
		switch request.Method {
		case "initialize":
			result = map[string]any{"userAgent": "fake-codex/1"}
		case "thread/start":
			threadNumber++
			var params struct {
				ApprovalPolicy string `json:"approvalPolicy"`
				Sandbox        string `json:"sandbox"`
			}
			_ = json.Unmarshal(request.Params, &params)
			if os.Getenv(fakePolicyMismatchEnv) == "1" {
				params.Sandbox = "danger-full-access"
			}
			result = map[string]any{
				"model":              "gpt-test",
				"modelProvider":      "openai",
				"approvalPolicy":     params.ApprovalPolicy,
				"sandbox":            map[string]any{"type": fakeSandboxPolicyType(params.Sandbox)},
				"instructionSources": []string{"/project/AGENTS.md"},
				"thread": map[string]any{
					"id":        fmt.Sprintf("thread-%d", threadNumber),
					"ephemeral": true,
				},
			}
		case "turn/start":
			turnNumber++
			if os.Getenv(fakeFloodEnv) == "1" {
				for range defaultNotificationLimit + 1 {
					writeFakeNotification("item/started", map[string]any{"sequence": turnNumber})
				}
			}
			result = map[string]any{"turn": map[string]any{"id": fmt.Sprintf("turn-%d", turnNumber)}}
		case "turn/interrupt":
			result = map[string]any{}
		default:
			writeFakeResponse(*request.ID, nil, map[string]any{"code": -32601, "message": "unknown method"})
			continue
		}
		writeFakeResponse(*request.ID, result, nil)
	}
}

// fakeSandboxPolicyType mirrors the generated app-server response schema rather than the request's CLI spelling.
func fakeSandboxPolicyType(policy string) string {
	switch policy {
	case "danger-full-access":
		return "dangerFullAccess"
	case "read-only":
		return "readOnly"
	case "workspace-write":
		return "workspaceWrite"
	default:
		return policy
	}
}

// writeFakeNotification emits one JSONL notification without a response identifier.
func writeFakeNotification(method string, params any) {
	encoded, _ := json.Marshal(map[string]any{"method": method, "params": params})
	_, _ = fmt.Fprintln(os.Stdout, string(encoded))
}

// writeFakeResponse emits one JSONL response without test-runner formatting.
func writeFakeResponse(id uint64, result any, protocolErr any) {
	response := map[string]any{"id": id}
	if protocolErr != nil {
		response["error"] = protocolErr
	} else {
		response["result"] = result
	}
	encoded, _ := json.Marshal(response)
	_, _ = fmt.Fprintln(os.Stdout, string(encoded))
}
