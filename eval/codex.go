package eval

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/goforj/atlas/internal/codexappserver"
)

const (
	adapterCleanupTimeout         = 5 * time.Second
	defaultProviderEventLimit     = 1024
	defaultProviderEventByteLimit = 8 << 20
)

// CodexOptions pins the Codex executable, model, provider state, and process environment used by diagnostic sessions.
type CodexOptions struct {
	Executable       string
	Arguments        []string
	Model            string
	ModelProvider    string
	Credential       CodexCredential
	CredentialSource string
	Environment      []string
}

// CodexCredential is an immutable credential snapshot shared by every treatment in one logical trial.
type CodexCredential struct {
	body             []byte
	digest           string
	redactionSecrets []string
}

// CodexAgent starts fresh Codex sessions and keeps preparation state private from generic runner contracts.
type CodexAgent struct {
	options    CodexOptions
	credential CodexCredential
	mu         sync.Mutex
	states     map[string]preparedState
}

// preparedState contains adapter-private inputs keyed by the runner-owned private home.
type preparedState struct {
	executable       string
	executableDigest string
	authorityDigest  string
	environment      RunEnvironment
	guidance         Guidance
}

// preparation references one adapter state entry until the runner closes it.
type preparation struct {
	adapter *CodexAgent
	agent   PreparedAgent
	once    sync.Once
}

// session owns one app-server client, fresh thread, and at most one turn.
type session struct {
	client         *codexappserver.Client
	thread         codexappserver.Thread
	identity       AgentSessionIdentity
	turn           codexappserver.Turn
	mu             sync.Mutex
	started        bool
	finished       bool
	sequence       uint64
	events         []Event
	eventLimit     int
	eventByteLimit int
	eventBytes     int
	telemetry      ProviderTelemetry
	message        string
	closeOnce      sync.Once
	closeErr       error
}

// NewCodexAgent creates a diagnostic Codex agent without claiming authoritative observation capabilities.
func NewCodexAgent(options CodexOptions) (*CodexAgent, error) {
	if strings.TrimSpace(options.Executable) == "" {
		return nil, fmt.Errorf("Codex executable is required")
	}
	if strings.TrimSpace(options.Model) == "" {
		return nil, fmt.Errorf("Codex model is required")
	}
	credential := options.Credential.clone()
	if len(credential.body) == 0 {
		var err error
		credential, err = LoadCodexCredential(options.CredentialSource)
		if err != nil {
			return nil, err
		}
	}
	options.Credential = CodexCredential{}
	options.CredentialSource = ""
	return &CodexAgent{options: options, credential: credential, states: map[string]preparedState{}}, nil
}

// LoadCodexCredential freezes one auth document before any treatment can begin.
func LoadCodexCredential(source string) (CodexCredential, error) {
	if strings.TrimSpace(source) == "" {
		return CodexCredential{}, fmt.Errorf("Codex credential source is required")
	}
	body, err := os.ReadFile(source)
	if err != nil {
		return CodexCredential{}, fmt.Errorf("read Codex credential: %w", err)
	}
	return NewCodexCredential(body)
}

// NewCodexCredential freezes credential bytes supplied by a host-owned read boundary.
func NewCodexCredential(body []byte) (CodexCredential, error) {
	if len(body) == 0 {
		return CodexCredential{}, fmt.Errorf("Codex credential is required")
	}
	var decoded any
	if err := json.Unmarshal(body, &decoded); err != nil {
		return CodexCredential{}, fmt.Errorf("decode Codex credential: %w", err)
	}
	hash := sha256.Sum256(body)
	return CodexCredential{
		body:             append([]byte(nil), body...),
		digest:           fmt.Sprintf("sha256:%x", hash[:]),
		redactionSecrets: codexCredentialSecrets(decoded, string(body)),
	}, nil
}

// Redactor returns a copy that covers the exact frozen authority used by Codex sessions.
func (credential CodexCredential) Redactor(redactor Redactor) Redactor {
	return redactor.withSecrets(credential.redactionSecrets)
}

// clone prevents caller-owned slices from changing authority or redaction after construction.
func (credential CodexCredential) clone() CodexCredential {
	return CodexCredential{
		body:             append([]byte(nil), credential.body...),
		digest:           credential.digest,
		redactionSecrets: append([]string(nil), credential.redactionSecrets...),
	}
}

// Name returns the adapter identity recorded by evaluation reports.
func (*CodexAgent) Name() string {
	return "codex"
}

// Properties returns no isolation guarantees because this diagnostic adapter shares the caller's operating-system identity.
func (*CodexAgent) Properties(context.Context) (AgentProperties, error) {
	return AgentProperties{}, nil
}

// Prepare fingerprints Codex and copies only the credential needed by the private app-server home.
func (adapter *CodexAgent) Prepare(_ context.Context, environment RunEnvironment, guidance Guidance) (AgentPreparation, error) {
	if adapter == nil {
		return nil, fmt.Errorf("Codex adapter is required")
	}
	if strings.TrimSpace(environment.ProjectRoot) == "" || strings.TrimSpace(environment.HomeRoot) == "" {
		return nil, fmt.Errorf("Codex Project and private home roots are required")
	}
	executable, digest, err := resolveExecutable(adapter.options.Executable)
	if err != nil {
		return nil, err
	}
	key := environment.HomeRoot
	adapter.mu.Lock()
	if _, exists := adapter.states[key]; exists {
		adapter.mu.Unlock()
		return nil, fmt.Errorf("Codex private home %q is already prepared", key)
	}
	adapter.states[key] = preparedState{}
	adapter.mu.Unlock()
	committed := false
	defer func() {
		if committed {
			return
		}
		adapter.mu.Lock()
		delete(adapter.states, key)
		adapter.mu.Unlock()
	}()
	if err := materializeCredential(environment.HomeRoot, adapter.credential.body); err != nil {
		return nil, err
	}
	state := preparedState{
		executable:       executable,
		executableDigest: digest,
		authorityDigest:  adapter.credential.digest,
		environment:      environment,
		guidance:         cloneGuidance(guidance),
	}
	adapter.mu.Lock()
	adapter.states[key] = state
	adapter.mu.Unlock()
	committed = true
	return &preparation{
		adapter: adapter,
		agent: PreparedAgent{
			Name:             adapter.Name(),
			Executable:       executable,
			ExecutableDigest: digest,
			AuthorityDigest:  adapter.credential.digest,
			Model:            adapter.options.Model,
			Environment:      environment,
		},
	}, nil
}

// Start launches app-server and rejects effective identity or instruction sources that differ from the prepared treatment.
func (adapter *CodexAgent) Start(ctx context.Context, agent PreparedAgent) (EvaluationSession, error) {
	if adapter == nil {
		return nil, fmt.Errorf("Codex adapter is required")
	}
	adapter.mu.Lock()
	state, ok := adapter.states[agent.Environment.HomeRoot]
	adapter.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("Codex preparation is unavailable for %q", agent.Environment.HomeRoot)
	}
	if agent.Name != adapter.Name() || agent.Executable != state.executable || agent.ExecutableDigest != state.executableDigest || agent.AuthorityDigest != state.authorityDigest || agent.Model != adapter.options.Model || agent.Environment.ProjectRoot != state.environment.ProjectRoot || agent.Environment.HomeRoot != state.environment.HomeRoot {
		return nil, fmt.Errorf("Codex prepared identity differs from adapter state")
	}
	processEnvironment := privateProcessEnvironment(mergeProcessEnvironment(adapter.options.Environment, state.environment.Environment), state.environment.HomeRoot)
	client, err := codexappserver.Start(ctx, codexappserver.StartOptions{
		Executable: state.executable,
		Arguments:  append([]string(nil), adapter.options.Arguments...),
		Dir:        state.environment.ProjectRoot,
		Env:        processEnvironment,
	})
	if err != nil {
		return nil, err
	}
	thread, err := client.StartThread(ctx, codexappserver.ThreadOptions{
		Model:          adapter.options.Model,
		ModelProvider:  adapter.options.ModelProvider,
		Dir:            state.environment.ProjectRoot,
		ApprovalPolicy: "never",
		Sandbox:        "danger-full-access",
		ServiceName:    "goforj_atlas_eval",
		Ephemeral:      true,
	})
	if err != nil {
		return nil, errors.Join(err, closeClient(client))
	}
	if thread.Model != adapter.options.Model || (adapter.options.ModelProvider != "" && thread.ModelProvider != adapter.options.ModelProvider) || !thread.Ephemeral {
		return nil, errors.Join(fmt.Errorf("Codex effective identity differs from prepared identity"), closeClient(client))
	}
	wantSources := guidanceSources(state.environment.ProjectRoot, state.guidance.Files)
	gotSources := normalizedPaths(thread.InstructionSources)
	if !equalStrings(gotSources, wantSources) {
		return nil, errors.Join(fmt.Errorf("Codex instruction sources = %q, want %q", gotSources, wantSources), closeClient(client))
	}
	return &session{
		client: client,
		thread: thread,
		identity: AgentSessionIdentity{
			Version:         client.UserAgent(),
			Model:           thread.Model,
			ModelProvider:   thread.ModelProvider,
			AuthorityDigest: state.authorityDigest,
		},
		eventLimit:     defaultProviderEventLimit,
		eventByteLimit: defaultProviderEventByteLimit,
	}, nil
}

// Agent returns the immutable identity passed back into CodexAgent.Start.
func (preparation *preparation) Agent() PreparedAgent {
	return preparation.agent
}

// Close drops adapter-private state while the backend retains ownership of filesystem cleanup.
func (preparation *preparation) Close(context.Context) error {
	if preparation == nil {
		return nil
	}
	preparation.once.Do(func() {
		preparation.adapter.mu.Lock()
		delete(preparation.adapter.states, preparation.agent.Environment.HomeRoot)
		preparation.adapter.mu.Unlock()
	})
	return nil
}

// Identity returns the effective agent and model identity established by the fresh app-server session.
func (session *session) Identity() AgentSessionIdentity {
	return session.identity
}

// Turn starts the single prompt accepted by a fresh diagnostic session.
func (session *session) Turn(ctx context.Context, turn AgentTurn) (AgentTurnResult, error) {
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.started {
		return AgentTurnResult{}, fmt.Errorf("Codex evaluation session accepts one turn")
	}
	started, err := session.client.StartTurn(ctx, session.thread.ID, turn.Prompt)
	if err != nil {
		return AgentTurnResult{}, err
	}
	session.turn = started
	session.started = true
	return AgentTurnResult{Accepted: true}, nil
}

// Wait drains protocol notifications until the active turn reaches a terminal state.
func (session *session) Wait(ctx context.Context) (AgentResult, error) {
	session.mu.Lock()
	if !session.started {
		session.mu.Unlock()
		return AgentResult{}, fmt.Errorf("Codex turn has not started")
	}
	if session.finished {
		session.mu.Unlock()
		return AgentResult{}, fmt.Errorf("Codex turn result was already consumed")
	}
	session.mu.Unlock()

	for {
		if err := session.telemetryOverflowError(); err != nil {
			return session.snapshotResult(AgentProviderError), err
		}
		select {
		case notification, ok := <-session.client.Notifications():
			if !ok {
				return session.snapshotResult(AgentProviderError), fmt.Errorf("Codex app-server stopped before turn completion: %s", session.client.Stderr())
			}
			terminal, outcome, err := session.consume(notification)
			if err != nil {
				return session.snapshotResult(AgentProviderError), err
			}
			if err := session.telemetryOverflowError(); err != nil {
				return session.snapshotResult(AgentProviderError), err
			}
			if terminal {
				session.mu.Lock()
				session.finished = true
				session.mu.Unlock()
				return session.snapshotResult(outcome), nil
			}
		case <-ctx.Done():
			return session.snapshotResult(classifyOperationError(ctx, ctx.Err(), AgentProviderError)), ctx.Err()
		}
	}
}

// snapshotResult preserves diagnostic telemetry collected before a provider or protocol failure.
func (session *session) snapshotResult(outcome AgentOutcome) AgentResult {
	session.mu.Lock()
	defer session.mu.Unlock()
	telemetry := session.telemetry
	telemetry.NotificationsDropped = session.client.NotificationsDropped()
	telemetry.NotificationBytesDropped = session.client.NotificationsDroppedBytes()
	return AgentResult{Outcome: outcome, Events: append([]Event(nil), session.events...), Message: session.message, Telemetry: &telemetry}
}

// telemetryOverflowError makes omitted provider evidence a deterministic terminal failure instead of a successful partial transcript.
func (session *session) telemetryOverflowError() error {
	session.mu.Lock()
	droppedEvents := session.telemetry.EventsDropped
	droppedEventBytes := session.telemetry.BytesDropped
	session.mu.Unlock()
	droppedNotifications := session.client.NotificationsDropped()
	droppedNotificationBytes := session.client.NotificationsDroppedBytes()
	if droppedEvents == 0 && droppedNotifications == 0 {
		return nil
	}
	return fmt.Errorf("Codex telemetry overflow: discarded %d retained events (%d bytes) and %d lifecycle notifications (%d bytes)", droppedEvents, droppedEventBytes, droppedNotifications, droppedNotificationBytes)
}

// Close interrupts an active turn before terminating the complete supervised app-server job.
func (session *session) Close(ctx context.Context) error {
	if session == nil || session.client == nil {
		return nil
	}
	session.closeOnce.Do(func() {
		session.mu.Lock()
		active := session.started && !session.finished
		threadID := session.thread.ID
		turnID := session.turn.ID
		session.mu.Unlock()
		var interruptErr error
		if active {
			interruptErr = session.client.InterruptTurn(ctx, threadID, turnID)
		}
		session.closeErr = errors.Join(interruptErr, session.client.Close(ctx))
	})
	return session.closeErr
}

// consume normalizes only the stable protocol subset needed by diagnostic evidence and terminal classification.
func (session *session) consume(notification codexappserver.Notification) (bool, AgentOutcome, error) {
	switch notification.Method {
	case "item/started", "item/completed":
		return false, "", session.consumeItem(notification.Method, notification.Params)
	case "turn/completed":
		var params struct {
			ThreadID string `json:"threadId"`
			Turn     struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"turn"`
		}
		if err := json.Unmarshal(notification.Params, &params); err != nil {
			return false, "", fmt.Errorf("decode Codex turn completion: %w", err)
		}
		if params.ThreadID != session.thread.ID || params.Turn.ID != session.turn.ID {
			return false, "", nil
		}
		session.appendEvent(EventRunFinished, map[string]string{"status": params.Turn.Status})
		switch params.Turn.Status {
		case "completed":
			return true, AgentCompleted, nil
		case "interrupted":
			return true, AgentCancelled, nil
		default:
			return true, AgentProviderError, nil
		}
	default:
		return false, "", nil
	}
}

// consumeItem records provider telemetry without upgrading it to trusted supervisor evidence.
func (session *session) consumeItem(method string, raw json.RawMessage) error {
	var params struct {
		ThreadID string          `json:"threadId"`
		TurnID   string          `json:"turnId"`
		Item     json.RawMessage `json:"item"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return fmt.Errorf("decode Codex item notification: %w", err)
	}
	if params.ThreadID != session.thread.ID || params.TurnID != session.turn.ID {
		return nil
	}
	var item struct {
		ID       string `json:"id"`
		Type     string `json:"type"`
		Command  string `json:"command"`
		CWD      string `json:"cwd"`
		ExitCode *int   `json:"exitCode"`
		Text     string `json:"text"`
		Server   string `json:"server"`
		Tool     string `json:"tool"`
		Status   string `json:"status"`
		Changes  []struct {
			Path string          `json:"path"`
			Kind json.RawMessage `json:"kind"`
		} `json:"changes"`
	}
	if err := json.Unmarshal(params.Item, &item); err != nil {
		return fmt.Errorf("decode Codex item: %w", err)
	}
	completed := method == "item/completed"
	switch item.Type {
	case "commandExecution":
		fields := map[string]string{EventFieldCommandID: item.ID, "command": item.Command, "cwd": item.CWD, "evidence": "provider-telemetry"}
		kind := EventCommandStarted
		if completed {
			kind = EventCommandFinished
			if item.ExitCode != nil {
				fields[EventFieldExitCode] = strconv.Itoa(*item.ExitCode)
			}
		}
		session.appendEvent(kind, fields)
	case "fileChange":
		if completed {
			for _, change := range item.Changes {
				session.appendEvent(EventFileWrite, map[string]string{"path": change.Path, "kind": normalizeFileChangeKind(change.Kind), "evidence": "provider-telemetry"})
			}
		}
	case "mcpToolCall":
		if completed {
			session.appendEvent(EventMCPToolCalled, map[string]string{"server": item.Server, "tool": item.Tool, "status": item.Status, "evidence": "provider-telemetry"})
		}
	case "agentMessage":
		if completed {
			if session.appendEvent(EventMessage, map[string]string{"text": item.Text}) {
				session.mu.Lock()
				session.message = item.Text
				session.mu.Unlock()
			}
		}
	}
	return nil
}

// normalizeFileChangeKind preserves useful telemetry across string and structured Codex protocol forms.
func normalizeFileChangeKind(raw json.RawMessage) string {
	var value string
	if json.Unmarshal(raw, &value) == nil {
		return value
	}
	var structured struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(raw, &structured) == nil && structured.Type != "" {
		return structured.Type
	}
	var compact bytes.Buffer
	if json.Compact(&compact, raw) == nil {
		return compact.String()
	}
	return "unknown"
}

// appendEvent assigns one adapter-local sequence while preserving app-server notification order.
func (session *session) appendEvent(kind EventKind, fields map[string]string) bool {
	session.mu.Lock()
	defer session.mu.Unlock()
	session.sequence++
	event := Event{Sequence: session.sequence, Kind: kind, Source: EventSourceAdapter, Time: time.Now().UTC(), Fields: fields}
	body, _ := json.Marshal(event)
	retainedBytes := len(body)
	session.telemetry.EventsObserved++
	session.telemetry.BytesObserved += uint64(retainedBytes)
	if kind == EventCommandStarted {
		session.telemetry.CommandsObserved++
	}
	if len(session.events) >= session.eventLimit || retainedBytes > session.eventByteLimit-session.eventBytes {
		session.telemetry.EventsDropped++
		session.telemetry.BytesDropped += uint64(retainedBytes)
		return false
	}
	session.events = append(session.events, event)
	session.eventBytes += retainedBytes
	return true
}

// resolveExecutable records the exact Codex binary rather than trusting later PATH lookup.
func resolveExecutable(candidate string) (string, string, error) {
	path, err := exec.LookPath(candidate)
	if err != nil {
		return "", "", fmt.Errorf("resolve Codex executable: %w", err)
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return "", "", fmt.Errorf("resolve absolute Codex executable: %w", err)
	}
	file, err := os.Open(path)
	if err != nil {
		return "", "", fmt.Errorf("open Codex executable: %w", err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", "", fmt.Errorf("digest Codex executable: %w", err)
	}
	return path, fmt.Sprintf("sha256:%x", hash.Sum(nil)), nil
}

// materializeGuidance writes only treatment-owned relative files and refuses to replace pre-existing Project content.
func materializeGuidance(root string, files map[string][]byte) ([]string, error) {
	created := make([]string, 0, len(files))
	for relative, body := range files {
		clean := filepath.Clean(relative)
		if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			removeGuidance(root, created)
			return nil, fmt.Errorf("invalid guidance path %q", relative)
		}
		path := filepath.Join(root, clean)
		if _, err := os.Lstat(path); err == nil {
			removeGuidance(root, created)
			return nil, fmt.Errorf("guidance path %q already exists", relative)
		} else if !os.IsNotExist(err) {
			removeGuidance(root, created)
			return nil, err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			removeGuidance(root, created)
			return nil, err
		}
		if err := os.WriteFile(path, body, 0o644); err != nil {
			removeGuidance(root, created)
			return nil, err
		}
		created = append(created, clean)
	}
	return created, nil
}

// removeGuidance rolls back only files created by a failed preparation and never removes user-authored parents.
func removeGuidance(root string, paths []string) {
	for _, relative := range paths {
		path := filepath.Join(root, relative)
		_ = os.Remove(path)
		for parent := filepath.Dir(path); parent != root && parent != filepath.Dir(parent); parent = filepath.Dir(parent) {
			if err := os.Remove(parent); err != nil {
				break
			}
		}
	}
}

// materializeCredential copies frozen authority into private state without inheriting normal user configuration.
func materializeCredential(homeRoot string, body []byte) error {
	if len(body) == 0 {
		return fmt.Errorf("Codex credential is required")
	}
	if err := os.MkdirAll(homeRoot, 0o700); err != nil {
		return err
	}
	path := filepath.Join(homeRoot, "auth.json")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		_ = os.Remove(path)
		return err
	}
	return nil
}

// codexCredentialSecrets collects secret-bearing string values and the exact serialized authority for report redaction.
func codexCredentialSecrets(value any, serialized string) []string {
	secrets := []string{serialized}
	seen := map[string]bool{serialized: true}
	var visit func(any)
	visit = func(current any) {
		switch typed := current.(type) {
		case []any:
			for _, item := range typed {
				visit(item)
			}
		case map[string]any:
			for key, item := range typed {
				if secret, ok := item.(string); ok && secret != "" && codexCredentialSecretField(key) && !seen[secret] {
					seen[secret] = true
					secrets = append(secrets, secret)
				}
				visit(item)
			}
		}
	}
	visit(value)
	return secrets
}

// codexCredentialSecretField recognizes authority values supported by Codex auth documents and provider extensions.
func codexCredentialSecretField(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	switch normalized {
	case "access_token", "token", "id_token", "refresh_token", "api_key", "client_secret", "password", "secret":
		return true
	default:
		return strings.HasSuffix(normalized, "_api_key") || strings.HasSuffix(normalized, "_access_token") || strings.HasSuffix(normalized, "_refresh_token")
	}
}

// privateProcessEnvironment isolates home and Codex configuration while retaining an explicit caller allowlist.
func privateProcessEnvironment(base []string, homeRoot string) []string {
	values := make(map[string]string, len(base)+2)
	for _, assignment := range base {
		key, value, ok := strings.Cut(assignment, "=")
		if ok && key != "HOME" && key != "CODEX_HOME" {
			values[key] = value
		}
	}
	values["HOME"] = homeRoot
	values["CODEX_HOME"] = homeRoot
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	environment := make([]string, 0, len(keys))
	for _, key := range keys {
		environment = append(environment, key+"="+values[key])
	}
	return environment
}

// mergeProcessEnvironment lets the backend's per-attempt isolation values override adapter-wide process defaults.
func mergeProcessEnvironment(base, attempt []string) []string {
	values := make(map[string]string, len(base)+len(attempt))
	for _, group := range [][]string{base, attempt} {
		for _, assignment := range group {
			key, value, ok := strings.Cut(assignment, "=")
			if ok {
				values[key] = value
			}
		}
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	merged := make([]string, 0, len(keys))
	for _, key := range keys {
		merged = append(merged, key+"="+values[key])
	}
	return merged
}

// guidanceSources resolves only Codex-recognized AGENTS files from the selected native treatment.
func guidanceSources(root string, files map[string][]byte) []string {
	var sources []string
	for relative := range files {
		if filepath.Base(relative) == "AGENTS.md" {
			sources = append(sources, filepath.Clean(filepath.Join(root, relative)))
		}
	}
	return normalizedPaths(sources)
}

// normalizedPaths produces a stable comparison independent from protocol ordering.
func normalizedPaths(paths []string) []string {
	normalized := make([]string, 0, len(paths))
	for _, path := range paths {
		absolute, err := filepath.Abs(path)
		if err == nil {
			normalized = append(normalized, filepath.Clean(absolute))
		}
	}
	sort.Strings(normalized)
	return normalized
}

// equalStrings compares instruction provenance after both sides have been normalized.
func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

// cloneGuidance protects adapter preparation from caller mutation during a live turn.
func cloneGuidance(guidance Guidance) Guidance {
	cloned := guidance
	cloned.Files = make(map[string][]byte, len(guidance.Files))
	for path, body := range guidance.Files {
		cloned.Files[path] = append([]byte(nil), body...)
	}
	cloned.Skills = append([]string(nil), guidance.Skills...)
	cloned.MCP = append([]string(nil), guidance.MCP...)
	return cloned
}

// closeClient gives failed startup an independent cleanup budget.
func closeClient(client *codexappserver.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), adapterCleanupTimeout)
	defer cancel()
	return client.Close(ctx)
}
