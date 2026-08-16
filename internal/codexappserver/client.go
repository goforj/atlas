// Package codexappserver provides the narrow Codex protocol boundary used by Atlas evaluations.
package codexappserver

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/goforj/atlas/internal/processgroup"
)

const (
	defaultMessageLimit      = 8 << 20
	defaultNotificationLimit = 1024
	cleanupTimeout           = 5 * time.Second
)

// StartOptions configures one private Codex app-server process.
type StartOptions struct {
	Executable string
	Arguments  []string
	Dir        string
	Env        []string
}

// ThreadOptions defines the attributable inputs for one fresh evaluation thread.
type ThreadOptions struct {
	Model           string
	ModelProvider   string
	Dir             string
	ApprovalPolicy  string
	Sandbox         string
	ServiceName     string
	Ephemeral       bool
	Configuration   map[string]any
	DeveloperPrompt string
}

// Thread describes the effective identity and guidance returned by Codex.
type Thread struct {
	ID                 string
	Model              string
	ModelProvider      string
	ApprovalPolicy     string
	Sandbox            string
	InstructionSources []string
	Ephemeral          bool
}

// Turn identifies one active request within a thread.
type Turn struct {
	ID string
}

// Notification is one app-server lifecycle message not paired with a client request.
type Notification struct {
	Method string
	Params json.RawMessage
}

// Client owns one initialized app-server connection and its complete process group.
type Client struct {
	process   *processgroup.Process
	stdin     *os.File
	userAgent string

	sendMu sync.Mutex
	nextID uint64

	pendingMu sync.Mutex
	pending   map[uint64]chan protocolMessage

	notifications        chan Notification
	notificationMu       sync.Mutex
	notificationsDropped uint64
	readerDone           chan struct{}
	readerMu             sync.Mutex
	readerErr            error
	stderr               lockedBuffer
	closeOnce            sync.Once
	closeDone            chan struct{}
	closeErr             error
}

// protocolMessage captures the common JSONL envelope without binding Atlas to the complete Codex schema.
type protocolMessage struct {
	ID     *uint64         `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *protocolError  `json:"error,omitempty"`
}

// protocolError is the stable JSON-RPC error subset needed for diagnostics.
type protocolError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// lockedBuffer bounds concurrent stderr access to the process lifetime.
type lockedBuffer struct {
	mu   sync.Mutex
	data []byte
}

// Start launches and initializes one Codex app-server connection.
func Start(ctx context.Context, options StartOptions) (*Client, error) {
	if options.Executable == "" {
		return nil, fmt.Errorf("Codex executable is required")
	}
	arguments := options.Arguments
	if len(arguments) == 0 {
		arguments = []string{"app-server", "--stdio", "--strict-config"}
	}

	stdinRead, stdinWrite, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("create Codex stdin: %w", err)
	}
	stdoutRead, stdoutWrite, err := os.Pipe()
	if err != nil {
		_ = stdinRead.Close()
		_ = stdinWrite.Close()
		return nil, fmt.Errorf("create Codex stdout: %w", err)
	}

	client := &Client{
		stdin:         stdinWrite,
		nextID:        1,
		pending:       map[uint64]chan protocolMessage{},
		notifications: make(chan Notification, defaultNotificationLimit),
		readerDone:    make(chan struct{}),
		closeDone:     make(chan struct{}),
	}
	process, err := processgroup.Start(options.Executable, arguments, processgroup.Options{
		Dir:    options.Dir,
		Env:    options.Env,
		Stdin:  stdinRead,
		Stdout: stdoutWrite,
		Stderr: &client.stderr,
	})
	_ = stdinRead.Close()
	_ = stdoutWrite.Close()
	if err != nil {
		_ = stdinWrite.Close()
		_ = stdoutRead.Close()
		return nil, err
	}
	client.process = process
	go client.read(stdoutRead)

	var initialized struct {
		UserAgent string `json:"userAgent"`
	}
	if err := client.request(ctx, "initialize", map[string]any{
		"clientInfo": map[string]string{
			"name":    "goforj_atlas_eval",
			"title":   "GoForj Atlas Eval",
			"version": "0.1.0",
		},
	}, &initialized); err != nil {
		return nil, errors.Join(err, client.closeAfterStartFailure())
	}
	if err := client.notify("initialized", map[string]any{}); err != nil {
		return nil, errors.Join(err, client.closeAfterStartFailure())
	}
	client.userAgent = initialized.UserAgent
	return client, nil
}

// UserAgent returns the provider version reported during the initialized protocol handshake.
func (client *Client) UserAgent() string {
	if client == nil {
		return ""
	}
	return client.userAgent
}

// StartThread creates a new thread and returns the effective identity reported by Codex.
func (client *Client) StartThread(ctx context.Context, options ThreadOptions) (Thread, error) {
	if options.Model == "" {
		return Thread{}, fmt.Errorf("Codex model is required")
	}
	params := map[string]any{
		"model":                      options.Model,
		"cwd":                        options.Dir,
		"approvalPolicy":             options.ApprovalPolicy,
		"sandbox":                    options.Sandbox,
		"serviceName":                options.ServiceName,
		"ephemeral":                  options.Ephemeral,
		"config":                     options.Configuration,
		"developerInstructions":      options.DeveloperPrompt,
		"allowProviderModelFallback": false,
	}
	if options.ModelProvider != "" {
		params["modelProvider"] = options.ModelProvider
	}

	var response struct {
		Model              string   `json:"model"`
		ModelProvider      string   `json:"modelProvider"`
		InstructionSources []string `json:"instructionSources"`
		Thread             struct {
			ID             string `json:"id"`
			Ephemeral      bool   `json:"ephemeral"`
			ApprovalPolicy string `json:"approvalPolicy"`
			Sandbox        string `json:"sandbox"`
		} `json:"thread"`
	}
	if err := client.request(ctx, "thread/start", params, &response); err != nil {
		return Thread{}, err
	}
	if response.Thread.ID == "" {
		return Thread{}, fmt.Errorf("Codex thread/start returned no thread id")
	}
	if response.Thread.ApprovalPolicy != options.ApprovalPolicy || response.Thread.Sandbox != options.Sandbox {
		return Thread{}, fmt.Errorf("Codex thread/start effective policy = approvalPolicy %q, sandbox %q; want approvalPolicy %q, sandbox %q", response.Thread.ApprovalPolicy, response.Thread.Sandbox, options.ApprovalPolicy, options.Sandbox)
	}
	return Thread{
		ID:                 response.Thread.ID,
		Model:              response.Model,
		ModelProvider:      response.ModelProvider,
		ApprovalPolicy:     response.Thread.ApprovalPolicy,
		Sandbox:            response.Thread.Sandbox,
		InstructionSources: append([]string(nil), response.InstructionSources...),
		Ephemeral:          response.Thread.Ephemeral,
	}, nil
}

// StartTurn submits one natural-language task to a fresh evaluation thread.
func (client *Client) StartTurn(ctx context.Context, threadID string, prompt string) (Turn, error) {
	if threadID == "" {
		return Turn{}, fmt.Errorf("Codex thread id is required")
	}
	if prompt == "" {
		return Turn{}, fmt.Errorf("Codex turn prompt is required")
	}
	var response struct {
		Turn struct {
			ID string `json:"id"`
		} `json:"turn"`
	}
	if err := client.request(ctx, "turn/start", map[string]any{
		"threadId": threadID,
		"input": []map[string]string{
			{"type": "text", "text": prompt},
		},
	}, &response); err != nil {
		return Turn{}, err
	}
	if response.Turn.ID == "" {
		return Turn{}, fmt.Errorf("Codex turn/start returned no turn id")
	}
	return Turn{ID: response.Turn.ID}, nil
}

// InterruptTurn asks Codex to cancel an active turn before process-group cleanup begins.
func (client *Client) InterruptTurn(ctx context.Context, threadID string, turnID string) error {
	if threadID == "" || turnID == "" {
		return fmt.Errorf("Codex thread and turn ids are required")
	}
	return client.request(ctx, "turn/interrupt", map[string]string{
		"threadId": threadID,
		"turnId":   turnID,
	}, nil)
}

// Notifications returns normalized server notifications in protocol order.
func (client *Client) Notifications() <-chan Notification {
	return client.notifications
}

// NotificationsDropped returns the number of lifecycle notifications discarded because the bounded telemetry queue was full.
func (client *Client) NotificationsDropped() uint64 {
	client.notificationMu.Lock()
	defer client.notificationMu.Unlock()
	return client.notificationsDropped
}

// Close terminates the complete app-server process group with a fresh cleanup budget.
func (client *Client) Close(ctx context.Context) error {
	if client == nil {
		return nil
	}
	client.closeOnce.Do(func() {
		go func() {
			_ = client.stdin.Close()
			client.closeErr = client.process.Terminate(ctx)
			close(client.closeDone)
		}()
	})
	select {
	case <-client.closeDone:
		return client.closeErr
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Stderr returns a snapshot of app-server diagnostics captured outside the candidate Project.
func (client *Client) Stderr() string {
	if client == nil {
		return ""
	}
	return client.stderr.String()
}

// request sends one JSON-RPC request and waits for its matching response while notifications continue streaming.
func (client *Client) request(ctx context.Context, method string, params any, result any) error {
	id := client.reserveID()
	response := make(chan protocolMessage, 1)
	client.pendingMu.Lock()
	client.pending[id] = response
	client.pendingMu.Unlock()

	if err := client.write(protocolMessage{ID: &id, Method: method}, params); err != nil {
		client.removePending(id)
		return err
	}
	select {
	case message := <-response:
		if message.Error != nil {
			return fmt.Errorf("Codex %s failed (%d): %s", method, message.Error.Code, message.Error.Message)
		}
		if result == nil || len(message.Result) == 0 {
			return nil
		}
		if err := json.Unmarshal(message.Result, result); err != nil {
			return fmt.Errorf("decode Codex %s response: %w", method, err)
		}
		return nil
	case <-client.readerDone:
		client.removePending(id)
		return fmt.Errorf("Codex app-server stopped during %s: %w", method, client.readError())
	case <-ctx.Done():
		client.removePending(id)
		return ctx.Err()
	}
}

// notify sends one client notification without allocating a response slot.
func (client *Client) notify(method string, params any) error {
	return client.write(protocolMessage{Method: method}, params)
}

// write serializes requests under one lock so concurrent turns cannot interleave JSONL frames.
func (client *Client) write(message protocolMessage, params any) error {
	body := map[string]any{"method": message.Method, "params": params}
	if message.ID != nil {
		body["id"] = *message.ID
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encode Codex %s request: %w", message.Method, err)
	}
	client.sendMu.Lock()
	defer client.sendMu.Unlock()
	if _, err := client.stdin.Write(append(encoded, '\n')); err != nil {
		return fmt.Errorf("write Codex %s request: %w", message.Method, err)
	}
	return nil
}

// read owns stdout framing so responses and lifecycle notifications cannot race each other.
func (client *Client) read(stdout *os.File) {
	defer close(client.readerDone)
	defer close(client.notifications)
	defer stdout.Close()
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), defaultMessageLimit)
	for scanner.Scan() {
		var message protocolMessage
		if err := json.Unmarshal(scanner.Bytes(), &message); err != nil {
			client.setReadError(fmt.Errorf("decode Codex app-server message: %w", err))
			return
		}
		if message.ID != nil && message.Method == "" {
			client.deliverResponse(*message.ID, message)
			continue
		}
		client.deliverNotification(Notification{Method: message.Method, Params: append(json.RawMessage(nil), message.Params...)})
	}
	if err := scanner.Err(); err != nil {
		client.setReadError(fmt.Errorf("read Codex app-server output: %w", err))
		return
	}
	client.setReadError(io.EOF)
}

// deliverNotification keeps protocol-response delivery independent from telemetry consumption.
func (client *Client) deliverNotification(notification Notification) {
	select {
	case client.notifications <- notification:
	default:
		client.notificationMu.Lock()
		client.notificationsDropped++
		client.notificationMu.Unlock()
	}
}

// deliverResponse removes a waiter before delivery so request cancellation cannot leave stale response state.
func (client *Client) deliverResponse(id uint64, message protocolMessage) {
	client.pendingMu.Lock()
	response, ok := client.pending[id]
	if ok {
		delete(client.pending, id)
	}
	client.pendingMu.Unlock()
	if ok {
		response <- message
	}
}

// removePending forgets a request whose caller can no longer receive its response.
func (client *Client) removePending(id uint64) {
	client.pendingMu.Lock()
	delete(client.pending, id)
	client.pendingMu.Unlock()
}

// reserveID returns a connection-local request identifier.
func (client *Client) reserveID() uint64 {
	client.sendMu.Lock()
	defer client.sendMu.Unlock()
	id := client.nextID
	client.nextID++
	return id
}

// setReadError records the first terminal framing result for request diagnostics.
func (client *Client) setReadError(err error) {
	client.readerMu.Lock()
	defer client.readerMu.Unlock()
	if client.readerErr == nil {
		client.readerErr = err
	}
}

// readError returns the terminal framing result after readerDone closes.
func (client *Client) readError() error {
	client.readerMu.Lock()
	defer client.readerMu.Unlock()
	return client.readerErr
}

// closeAfterStartFailure prevents a failed handshake from leaking app-server descendants.
func (client *Client) closeAfterStartFailure() error {
	ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
	defer cancel()
	return client.Close(ctx)
}

// Write appends stderr bytes while the process is active.
func (buffer *lockedBuffer) Write(data []byte) (int, error) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	buffer.data = append(buffer.data, data...)
	return len(data), nil
}

// String returns an immutable stderr snapshot for diagnostics.
func (buffer *lockedBuffer) String() string {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return string(append([]byte(nil), buffer.data...))
}
