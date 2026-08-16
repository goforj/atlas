package eval

import (
	"bytes"
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
const maxArtifactManifestSize = 1 << 20

const artifactManifestName = "manifest.json"

var artifactAttemptIDPattern = regexp.MustCompile(`^[A-Za-z0-9]+(?:[-_][A-Za-z0-9]+)*$`)

var allowedArtifactFileNames = [...]string{
	"run.json",
	"scorecard.json",
	"summary.txt",
	"events.jsonl",
	"transcript.redacted.txt",
	"commands.jsonl",
	"diff.patch",
	"verification.json",
	"environment.json",
	"triage.json",
}

const maxArtifactDirectoryEntries = len(allowedArtifactFileNames) + 1

var allowedArtifactFiles = func() map[string]bool {
	files := make(map[string]bool, len(allowedArtifactFileNames))
	for _, name := range allowedArtifactFileNames {
		files[name] = true
	}
	return files
}()

// artifactDirectory holds one descriptor-anchored view of an attempt directory and its bounded entries.
type artifactDirectory struct {
	root    *os.Root
	entries map[string]os.FileInfo
}

// ArtifactStore creates private supervisor-owned attempt directories and manifests with post-run tamper evidence.
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

// ArtifactManifest integrity-checks the complete retained evidence set against later accidental changes.
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
	directory         string
	directoryIdentity os.FileInfo
	attemptID         string
	key               []byte
	redactor          Redactor
	events            *os.File
	lastEvent         uint64
	closed            bool
}

// NewArtifactStore requires an integrity key rather than writing unsigned evidence.
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
	if err := prepareArtifactRoot(store.root); err != nil {
		return nil, err
	}
	directory := filepath.Join(store.root, attemptID)
	if err := os.Mkdir(directory, 0o700); err != nil {
		return nil, fmt.Errorf("create attempt artifacts: %w", err)
	}
	events, err := os.OpenFile(filepath.Join(directory, "events.jsonl"), os.O_CREATE|os.O_EXCL|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create event artifact: %w", err)
	}
	directoryIdentity, err := os.Stat(directory)
	if err != nil {
		_ = events.Close()
		_ = os.RemoveAll(directory)
		return nil, fmt.Errorf("inspect attempt artifacts: %w", err)
	}
	return &AttemptArtifacts{
		directory:         directory,
		directoryIdentity: directoryIdentity,
		attemptID:         attemptID,
		key:               append([]byte(nil), store.key...),
		redactor:          store.redactor,
		events:            events,
	}, nil
}

// prepareArtifactRoot creates a private root or verifies that an existing root is already safe to trust.
func prepareArtifactRoot(root string) error {
	if err := os.Mkdir(root, 0o700); err != nil && !os.IsExist(err) {
		return fmt.Errorf("create artifact root: %w", err)
	}
	rootInfo, err := os.Lstat(root)
	if err != nil {
		return fmt.Errorf("inspect artifact root: %w", err)
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return fmt.Errorf("artifact root must be a real directory")
	}
	if rootInfo.Mode().Perm() != 0o700 {
		return fmt.Errorf("artifact root permissions must be 0700")
	}
	return nil
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
	directory, err := openArtifactDirectory(artifacts.directory)
	if err != nil {
		return ArtifactManifest{}, err
	}
	defer directory.close()
	if _, exists := directory.entries[artifactManifestName]; exists {
		return ArtifactManifest{}, fmt.Errorf("artifact manifest already exists")
	}
	files, err := collectArtifactFiles(directory, artifacts.inspectRetainedArtifact)
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
	if err := writeArtifactManifest(directory.root, body); err != nil {
		return ArtifactManifest{}, fmt.Errorf("write artifact manifest: %w", err)
	}
	return manifest, nil
}

// VerifyArtifactManifest authenticates metadata and every retained file without executing artifact content.
func VerifyArtifactManifest(directory string, key []byte) (ArtifactManifest, error) {
	if err := validateArtifactAuthenticationKey(key); err != nil {
		return ArtifactManifest{}, err
	}
	artifactRoot, err := openArtifactDirectory(directory)
	if err != nil {
		return ArtifactManifest{}, err
	}
	defer artifactRoot.close()
	body, err := readArtifactManifest(artifactRoot)
	if err != nil {
		return ArtifactManifest{}, err
	}
	var manifest ArtifactManifest
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return ArtifactManifest{}, fmt.Errorf("decode artifact manifest: %w", err)
	}
	if err := requireJSONEnd(decoder); err != nil {
		return ArtifactManifest{}, fmt.Errorf("decode artifact manifest: %w", err)
	}
	canonical, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return ArtifactManifest{}, fmt.Errorf("encode canonical artifact manifest: %w", err)
	}
	canonical = append(canonical, '\n')
	if !bytes.Equal(body, canonical) {
		return ArtifactManifest{}, fmt.Errorf("artifact manifest is not in canonical form")
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
	actual, err := collectArtifactFiles(artifactRoot, nil)
	if err != nil {
		return ArtifactManifest{}, err
	}
	if !equalArtifactFiles(actual, manifest.Files) {
		return ArtifactManifest{}, fmt.Errorf("artifact file identities do not match the manifest")
	}
	return manifest, nil
}

// readArtifactManifest bounds and validates the manifest file before parsing attacker-controlled evidence.
func readArtifactManifest(directory *artifactDirectory) ([]byte, error) {
	pathInfo, exists := directory.entries[artifactManifestName]
	if !exists {
		return nil, fmt.Errorf("inspect artifact manifest: %s is missing", artifactManifestName)
	}
	file, err := openArtifactFile(directory.root, artifactManifestName, pathInfo)
	if err != nil {
		return nil, fmt.Errorf("inspect artifact manifest: %w", err)
	}
	body, readErr := io.ReadAll(io.LimitReader(file, maxArtifactManifestSize+1))
	closeErr := file.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return nil, fmt.Errorf("read artifact manifest: %w", err)
	}
	if len(body) > maxArtifactManifestSize {
		return nil, fmt.Errorf("artifact manifest exceeds %d bytes", maxArtifactManifestSize)
	}
	return body, nil
}

// openArtifactDirectory anchors all later access to one directory descriptor and rejects an unbounded or unsupported surface.
func openArtifactDirectory(path string) (*artifactDirectory, error) {
	pathInfo, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("inspect artifact directory: %w", err)
	}
	if pathInfo.Mode()&os.ModeSymlink != 0 || !pathInfo.IsDir() {
		return nil, fmt.Errorf("artifact directory must be a real directory")
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, fmt.Errorf("open artifact directory: %w", err)
	}
	rootInfo, err := root.Stat(".")
	if err != nil {
		_ = root.Close()
		return nil, fmt.Errorf("inspect opened artifact directory: %w", err)
	}
	if !rootInfo.IsDir() || !os.SameFile(pathInfo, rootInfo) {
		_ = root.Close()
		return nil, fmt.Errorf("artifact directory changed while opening")
	}
	entries, err := readArtifactDirectoryEntries(root)
	if err != nil {
		_ = root.Close()
		return nil, err
	}
	return &artifactDirectory{root: root, entries: entries}, nil
}

// readArtifactDirectoryEntries reads at most one entry beyond the fixed artifact surface before rejecting it.
func readArtifactDirectoryEntries(root *os.Root) (map[string]os.FileInfo, error) {
	directory, err := root.Open(".")
	if err != nil {
		return nil, fmt.Errorf("open artifact directory snapshot: %w", err)
	}
	entries, readErr := directory.ReadDir(maxArtifactDirectoryEntries + 1)
	closeErr := directory.Close()
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return nil, fmt.Errorf("read artifact directory: %w", readErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close artifact directory snapshot: %w", closeErr)
	}
	if len(entries) > maxArtifactDirectoryEntries {
		return nil, fmt.Errorf("artifact directory exceeds %d entries", maxArtifactDirectoryEntries)
	}
	result := make(map[string]os.FileInfo, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if name != artifactManifestName && !allowedArtifactFiles[name] {
			return nil, fmt.Errorf("artifact %q is not in the allowed artifact surface", name)
		}
		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("inspect artifact %s: %w", name, err)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("artifact %s is not a regular file", name)
		}
		result[name] = info
	}
	return result, nil
}

// openArtifactFile opens nonblockingly and verifies that the descriptor is the enumerated regular file.
func openArtifactFile(root *os.Root, name string, pathInfo os.FileInfo) (*os.File, error) {
	file, err := openArtifactFileDescriptor(root, name)
	if err != nil {
		return nil, fmt.Errorf("open artifact %s: %w", name, err)
	}
	fileInfo, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("inspect opened artifact %s: %w", name, err)
	}
	if !fileInfo.Mode().IsRegular() || !os.SameFile(pathInfo, fileInfo) {
		_ = file.Close()
		return nil, fmt.Errorf("artifact %s changed while opening", name)
	}
	return file, nil
}

// writeArtifactManifest creates the final manifest through the anchored root without replacing an unexpected entry.
func writeArtifactManifest(root *os.Root, body []byte) error {
	file, err := root.OpenFile(artifactManifestName, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(body)
	closeErr := file.Close()
	return errors.Join(writeErr, closeErr)
}

// close releases the descriptor-anchored artifact directory.
func (directory *artifactDirectory) close() {
	_ = directory.root.Close()
}

// requireJSONEnd rejects concatenated values that a single decoder call would otherwise ignore.
func requireJSONEnd(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("multiple JSON values are not allowed")
		}
		return err
	}
	return nil
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

// inspectRetainedArtifact fails finalization if a file bypassed normal redacted write paths.
func (artifacts *AttemptArtifacts) inspectRetainedArtifact(name string, body []byte) error {
	if artifacts.redactor.containsSecret(name) {
		return fmt.Errorf("artifact contains a registered secret")
	}
	return artifacts.rejectRegisteredSecrets(body)
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

// collectArtifactFiles hashes bounded regular files and optionally inspects the exact bytes being authenticated.
func collectArtifactFiles(directory *artifactDirectory, inspect func(string, []byte) error) ([]ArtifactFile, error) {
	files := make([]ArtifactFile, 0, len(directory.entries))
	var totalSize int64
	for name, info := range directory.entries {
		if name == artifactManifestName {
			continue
		}
		digest, size, err := digestArtifactFile(directory.root, name, info, inspect)
		if err != nil {
			return nil, err
		}
		totalSize += size
		if totalSize > maxArtifactTotalSize {
			return nil, fmt.Errorf("artifact set exceeds %d bytes", maxArtifactTotalSize)
		}
		files = append(files, ArtifactFile{Path: name, Digest: digest, Size: size, Classification: "diagnostic"})
	}
	sort.Slice(files, func(left, right int) bool { return files[left].Path < files[right].Path })
	return files, nil
}

// digestArtifactFile hashes a regular file without executing its inert contents.
func digestArtifactFile(root *os.Root, name string, pathInfo os.FileInfo, inspect func(string, []byte) error) (string, int64, error) {
	file, err := openArtifactFile(root, name, pathInfo)
	if err != nil {
		return "", 0, err
	}
	body, readErr := io.ReadAll(io.LimitReader(file, maxArtifactFileSize+1))
	closeErr := file.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return "", 0, fmt.Errorf("digest artifact %s: %w", name, err)
	}
	if len(body) > maxArtifactFileSize {
		return "", 0, fmt.Errorf("artifact %s exceeds %d bytes", name, maxArtifactFileSize)
	}
	if inspect != nil {
		if err := inspect(name, body); err != nil {
			return "", 0, err
		}
	}
	digest := sha256.Sum256(body)
	return fmt.Sprintf("sha256:%x", digest), int64(len(body)), nil
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
