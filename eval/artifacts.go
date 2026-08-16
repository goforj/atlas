package eval

import (
	"bufio"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

const artifactManifestSchemaVersion = 1

const maxArtifactFileSize = 16 << 20
const maxArtifactTotalSize = 64 << 20

var artifactAttemptIDPattern = regexp.MustCompile(`^[A-Za-z0-9]+(?:[-_][A-Za-z0-9]+)*$`)

var allowedArtifactFiles = map[string]bool{
	"run.json":                true,
	"scorecard.json":          true,
	"summary.txt":             true,
	"events.jsonl":            true,
	"transcript.redacted.txt": true,
	"commands.jsonl":          true,
	"diff.patch":              true,
	"verification.json":       true,
	"environment.json":        true,
	"triage.json":             true,
}

// ArtifactStore creates private supervisor-owned attempt directories and authenticated manifests.
type ArtifactStore struct {
	root     string
	key      []byte
	redactor Redactor
}

// ArtifactFile records one retained file's exact identity and classification.
type ArtifactFile struct {
	Path           string `json:"path"`
	Digest         string `json:"digest"`
	Size           int64  `json:"size"`
	Classification string `json:"classification"`
}

// ArtifactManifest authenticates the complete retained evidence set.
type ArtifactManifest struct {
	SchemaVersion int            `json:"schema_version"`
	AttemptID     string         `json:"attempt_id"`
	PlanDigest    string         `json:"plan_digest"`
	BaselineTree  string         `json:"baseline_tree"`
	FinalTree     string         `json:"final_tree"`
	Files         []ArtifactFile `json:"files"`
	Signature     string         `json:"signature"`
}

// AttemptArtifacts owns append-only evidence for one attempt until finalization.
type AttemptArtifacts struct {
	directory string
	attemptID string
	key       []byte
	redactor  Redactor
	events    *os.File
	lastEvent uint64
	closed    bool
}

// NewArtifactStore requires an authentication key rather than writing unsigned evidence.
func NewArtifactStore(root string, key []byte, redactor Redactor) (*ArtifactStore, error) {
	if root == "" {
		return nil, fmt.Errorf("artifact root is required")
	}
	if err := validateArtifactAuthenticationKey(key); err != nil {
		return nil, err
	}
	return &ArtifactStore{root: root, key: append([]byte(nil), key...), redactor: redactor}, nil
}

// Begin creates one private attempt directory outside any agent-provided path.
func (store *ArtifactStore) Begin(attemptID string) (*AttemptArtifacts, error) {
	if store == nil {
		return nil, fmt.Errorf("artifact store is required")
	}
	if !artifactAttemptIDPattern.MatchString(attemptID) {
		return nil, fmt.Errorf("attempt ID %q must be a safe slug", attemptID)
	}
	if err := os.MkdirAll(store.root, 0o700); err != nil {
		return nil, fmt.Errorf("create artifact root: %w", err)
	}
	if err := os.Chmod(store.root, 0o700); err != nil {
		return nil, fmt.Errorf("secure artifact root: %w", err)
	}
	rootInfo, err := os.Lstat(store.root)
	if err != nil {
		return nil, fmt.Errorf("inspect artifact root: %w", err)
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return nil, fmt.Errorf("artifact root must be a real directory")
	}
	directory := filepath.Join(store.root, attemptID)
	if err := os.Mkdir(directory, 0o700); err != nil {
		return nil, fmt.Errorf("create attempt artifacts: %w", err)
	}
	events, err := os.OpenFile(filepath.Join(directory, "events.jsonl"), os.O_CREATE|os.O_EXCL|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create event artifact: %w", err)
	}
	return &AttemptArtifacts{
		directory: directory,
		attemptID: attemptID,
		key:       append([]byte(nil), store.key...),
		redactor:  store.redactor,
		events:    events,
	}, nil
}

// AppendEvent redacts and persists one event before accepting the next sequence item.
func (artifacts *AttemptArtifacts) AppendEvent(event Event) error {
	if artifacts == nil || artifacts.closed {
		return fmt.Errorf("attempt artifacts are closed")
	}
	if event.Sequence == 0 || event.Sequence <= artifacts.lastEvent {
		return fmt.Errorf("event sequence %d must be greater than %d", event.Sequence, artifacts.lastEvent)
	}
	body, err := json.Marshal(artifacts.redactor.Event(event))
	if err != nil {
		return fmt.Errorf("encode event: %w", err)
	}
	body = append(body, '\n')
	if err := artifacts.rejectRegisteredSecrets(body); err != nil {
		return err
	}
	info, err := artifacts.events.Stat()
	if err != nil {
		return fmt.Errorf("inspect event artifact: %w", err)
	}
	if info.Size()+int64(len(body)) > maxArtifactFileSize {
		return fmt.Errorf("event artifact exceeds %d bytes", maxArtifactFileSize)
	}
	if _, err := artifacts.events.Write(body); err != nil {
		return fmt.Errorf("append event: %w", err)
	}
	if err := artifacts.events.Sync(); err != nil {
		return err
	}
	artifacts.lastEvent = event.Sequence
	return nil
}

// WriteText writes one bounded inert text artifact selected by the supervisor.
func (artifacts *AttemptArtifacts) WriteText(name, content string) error {
	if err := artifacts.validateArtifactName(name); err != nil {
		return err
	}
	body := []byte(artifacts.redactor.Text(content))
	if err := artifacts.rejectRegisteredSecrets(body); err != nil {
		return err
	}
	if len(body) > maxArtifactFileSize {
		return fmt.Errorf("artifact %s exceeds %d bytes", name, maxArtifactFileSize)
	}
	return os.WriteFile(filepath.Join(artifacts.directory, name), body, 0o600)
}

// WriteJSON writes one typed artifact after recursively redacting its serialized representation.
func (artifacts *AttemptArtifacts) WriteJSON(name string, value any) error {
	if err := artifacts.validateArtifactName(name); err != nil {
		return err
	}
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", name, err)
	}
	var decoded any
	if err := json.Unmarshal(body, &decoded); err != nil {
		return fmt.Errorf("normalize %s: %w", name, err)
	}
	body, err = json.MarshalIndent(artifacts.redactor.JSONValue(decoded), "", "  ")
	if err != nil {
		return fmt.Errorf("redact %s: %w", name, err)
	}
	body = append(body, '\n')
	if err := artifacts.rejectRegisteredSecrets(body); err != nil {
		return err
	}
	if len(body) > maxArtifactFileSize {
		return fmt.Errorf("artifact %s exceeds %d bytes", name, maxArtifactFileSize)
	}
	return os.WriteFile(filepath.Join(artifacts.directory, name), body, 0o600)
}

// Finalize closes streaming evidence and writes the authenticated manifest last.
func (artifacts *AttemptArtifacts) Finalize(planDigest, baselineTree, finalTree string) (ArtifactManifest, error) {
	if artifacts == nil || artifacts.closed {
		return ArtifactManifest{}, fmt.Errorf("attempt artifacts are closed")
	}
	if err := artifacts.events.Close(); err != nil {
		return ArtifactManifest{}, fmt.Errorf("close event artifact: %w", err)
	}
	artifacts.closed = true
	if err := artifacts.scanRetainedArtifacts(); err != nil {
		return ArtifactManifest{}, err
	}
	files, err := collectArtifactFiles(artifacts.directory)
	if err != nil {
		return ArtifactManifest{}, err
	}
	manifest := ArtifactManifest{
		SchemaVersion: artifactManifestSchemaVersion,
		AttemptID:     artifacts.attemptID,
		PlanDigest:    planDigest,
		BaselineTree:  baselineTree,
		FinalTree:     finalTree,
		Files:         files,
	}
	signature, err := signArtifactManifest(manifest, artifacts.key)
	if err != nil {
		return ArtifactManifest{}, err
	}
	manifest.Signature = signature
	body, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return ArtifactManifest{}, fmt.Errorf("encode artifact manifest: %w", err)
	}
	body = append(body, '\n')
	if err := artifacts.rejectRegisteredSecrets(body); err != nil {
		return ArtifactManifest{}, err
	}
	if err := os.WriteFile(filepath.Join(artifacts.directory, "manifest.json"), body, 0o600); err != nil {
		return ArtifactManifest{}, fmt.Errorf("write artifact manifest: %w", err)
	}
	return manifest, nil
}

// VerifyArtifactManifest authenticates metadata and every retained file without executing artifact content.
func VerifyArtifactManifest(directory string, key []byte) (ArtifactManifest, error) {
	if err := validateArtifactAuthenticationKey(key); err != nil {
		return ArtifactManifest{}, err
	}
	body, err := os.ReadFile(filepath.Join(directory, "manifest.json"))
	if err != nil {
		return ArtifactManifest{}, fmt.Errorf("read artifact manifest: %w", err)
	}
	var manifest ArtifactManifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return ArtifactManifest{}, fmt.Errorf("decode artifact manifest: %w", err)
	}
	if manifest.SchemaVersion != artifactManifestSchemaVersion {
		return ArtifactManifest{}, fmt.Errorf("unsupported artifact manifest schema %d", manifest.SchemaVersion)
	}
	signature := manifest.Signature
	manifest.Signature = ""
	want, err := signArtifactManifest(manifest, key)
	if err != nil {
		return ArtifactManifest{}, err
	}
	if !hmac.Equal([]byte(signature), []byte(want)) {
		return ArtifactManifest{}, fmt.Errorf("artifact manifest signature is invalid")
	}
	manifest.Signature = signature
	actual, err := collectArtifactFiles(directory)
	if err != nil {
		return ArtifactManifest{}, err
	}
	if !equalArtifactFiles(actual, manifest.Files) {
		return ArtifactManifest{}, fmt.Errorf("artifact file identities do not match the manifest")
	}
	return manifest, nil
}

// validateArtifactAuthenticationKey rejects keys too weak to authenticate retained evidence.
func validateArtifactAuthenticationKey(key []byte) error {
	if len(key) < 32 {
		return fmt.Errorf("artifact authentication key must contain at least 32 bytes")
	}
	return nil
}

// rejectRegisteredSecrets prevents an incomplete redaction from reaching durable storage.
func (artifacts *AttemptArtifacts) rejectRegisteredSecrets(body []byte) error {
	if artifacts.redactor.containsSecret(string(body)) {
		return fmt.Errorf("artifact contains a registered secret")
	}
	return nil
}

// scanRetainedArtifacts fails finalization if a file bypassed normal redacted write paths.
func (artifacts *AttemptArtifacts) scanRetainedArtifacts() error {
	entries, err := os.ReadDir(artifacts.directory)
	if err != nil {
		return fmt.Errorf("read artifact directory: %w", err)
	}
	for _, entry := range entries {
		if entry.Name() == "manifest.json" {
			continue
		}
		if artifacts.redactor.containsSecret(entry.Name()) {
			return fmt.Errorf("artifact contains a registered secret")
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("inspect artifact %s: %w", entry.Name(), err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("artifact %s is not a regular file", entry.Name())
		}
		file, err := os.Open(filepath.Join(artifacts.directory, entry.Name()))
		if err != nil {
			return fmt.Errorf("open artifact %s: %w", entry.Name(), err)
		}
		body, readErr := io.ReadAll(io.LimitReader(file, maxArtifactFileSize+1))
		closeErr := file.Close()
		if err := errors.Join(readErr, closeErr); err != nil {
			return fmt.Errorf("read artifact %s: %w", entry.Name(), err)
		}
		if len(body) > maxArtifactFileSize {
			return fmt.Errorf("artifact %s exceeds %d bytes", entry.Name(), maxArtifactFileSize)
		}
		if err := artifacts.rejectRegisteredSecrets(body); err != nil {
			return err
		}
	}
	return nil
}

// validateArtifactName confines writes to the fixed evidence surface.
func (artifacts *AttemptArtifacts) validateArtifactName(name string) error {
	if artifacts == nil || artifacts.closed {
		return fmt.Errorf("attempt artifacts are closed")
	}
	if !allowedArtifactFiles[name] || name == "events.jsonl" {
		return fmt.Errorf("artifact name %q is not writable through this operation", name)
	}
	return nil
}

// collectArtifactFiles hashes bounded regular files and rejects links or unsupported entries.
func collectArtifactFiles(directory string) ([]ArtifactFile, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("read artifact directory: %w", err)
	}
	files := make([]ArtifactFile, 0, len(entries))
	var totalSize int64
	for _, entry := range entries {
		if entry.Name() == "manifest.json" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("inspect artifact %s: %w", entry.Name(), err)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("artifact %s is not a regular file", entry.Name())
		}
		digest, size, err := digestArtifactFile(filepath.Join(directory, entry.Name()))
		if err != nil {
			return nil, err
		}
		totalSize += size
		if totalSize > maxArtifactTotalSize {
			return nil, fmt.Errorf("artifact set exceeds %d bytes", maxArtifactTotalSize)
		}
		files = append(files, ArtifactFile{Path: entry.Name(), Digest: digest, Size: size, Classification: "diagnostic"})
	}
	sort.Slice(files, func(left, right int) bool { return files[left].Path < files[right].Path })
	return files, nil
}

// digestArtifactFile hashes a regular file without interpreting its inert contents.
func digestArtifactFile(path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, fmt.Errorf("open artifact %s: %w", filepath.Base(path), err)
	}
	defer file.Close()
	hash := sha256.New()
	size, err := io.Copy(hash, io.LimitReader(bufio.NewReader(file), maxArtifactFileSize+1))
	if err != nil {
		return "", 0, fmt.Errorf("digest artifact %s: %w", filepath.Base(path), err)
	}
	if size > maxArtifactFileSize {
		return "", 0, fmt.Errorf("artifact %s exceeds %d bytes", filepath.Base(path), maxArtifactFileSize)
	}
	return fmt.Sprintf("sha256:%x", hash.Sum(nil)), size, nil
}

// signArtifactManifest authenticates canonical JSON without including the signature field itself.
func signArtifactManifest(manifest ArtifactManifest, key []byte) (string, error) {
	manifest.Signature = ""
	body, err := json.Marshal(manifest)
	if err != nil {
		return "", fmt.Errorf("encode manifest signature payload: %w", err)
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(body)
	return fmt.Sprintf("hmac-sha256:%x", mac.Sum(nil)), nil
}

// equalArtifactFiles compares the canonical sorted identity set.
func equalArtifactFiles(left, right []ArtifactFile) bool {
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
