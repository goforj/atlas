package eval

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestArtifactStoreRedactsBeforePersistenceAndAuthenticatesFiles verifies the supervisor evidence boundary end to end.
func TestArtifactStoreRedactsBeforePersistenceAndAuthenticatesFiles(t *testing.T) {
	root := filepath.Join(t.TempDir(), "artifacts")
	key := []byte("0123456789abcdef0123456789abcdef")
	secret := "provider-secret-canary"
	store, err := NewArtifactStore(root, key, NewRedactor([]string{secret}))
	if err != nil {
		t.Fatalf("NewArtifactStore(): %v", err)
	}
	artifacts, err := store.Begin("attempt-01")
	if err != nil {
		t.Fatalf("Begin(): %v", err)
	}
	if err := artifacts.AppendEvent(Event{
		Sequence: 1,
		Kind:     EventMessage,
		Time:     time.Unix(1_700_000_000, 0).UTC(),
		Fields: map[string]string{
			"message":        "Authorization: " + secret,
			"provider_token": secret,
		},
	}); err != nil {
		t.Fatalf("AppendEvent(): %v", err)
	}
	if err := artifacts.WriteText("transcript.redacted.txt", "\x1b[31mBearer "+secret+"\x1b[0m\u202esecret"); err != nil {
		t.Fatalf("WriteText(): %v", err)
	}
	result := AttemptResult{AttemptID: "attempt-01", SecondaryFailures: []SecondaryFailure{{Phase: "provider", Message: "token=" + secret}}}
	if err := artifacts.WriteJSON("run.json", result); err != nil {
		t.Fatalf("WriteJSON(): %v", err)
	}
	manifest, err := artifacts.Finalize("sha256:plan", "sha256:baseline", "sha256:final")
	if err != nil {
		t.Fatalf("Finalize(): %v", err)
	}
	if manifest.Signature == "" || len(manifest.Files) != 3 {
		t.Fatalf("artifact manifest = %#v", manifest)
	}
	directory := filepath.Join(root, "attempt-01")
	if _, err := VerifyArtifactManifest(directory, key); err != nil {
		t.Fatalf("VerifyArtifactManifest(): %v", err)
	}
	if err := filepath.WalkDir(directory, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(body), secret) || strings.Contains(string(body), "\x1b") || strings.Contains(string(body), "\u202e") {
			t.Fatalf("unsafe evidence persisted in %s: %q", path, body)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(directory)
	if err != nil {
		t.Fatalf("stat artifact directory: %v", err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("artifact directory permissions = %o, want 700", info.Mode().Perm())
	}
}

// TestArtifactStoreRejectsAgentControlledPaths keeps artifact writes on a fixed supervisor-owned surface.
func TestArtifactStoreRejectsAgentControlledPaths(t *testing.T) {
	store, err := NewArtifactStore(filepath.Join(t.TempDir(), "artifacts"), []byte("0123456789abcdef0123456789abcdef"), NewRedactor(nil))
	if err != nil {
		t.Fatalf("NewArtifactStore(): %v", err)
	}
	if _, err := store.Begin("../escape"); err == nil {
		t.Fatal("Begin() accepted a traversal attempt ID")
	}
	artifacts, err := store.Begin("attempt-02")
	if err != nil {
		t.Fatalf("Begin(): %v", err)
	}
	if err := artifacts.WriteText("../outside", "content"); err == nil {
		t.Fatal("WriteText() accepted an arbitrary path")
	}
	if _, err := artifacts.Finalize("sha256:plan", "sha256:baseline", "sha256:final"); err != nil {
		t.Fatalf("Finalize(): %v", err)
	}
}

// TestArtifactStoreCreatesPrivateRootOrRejectsUnsafeExistingRoots keeps evidence outside agent-controlled locations.
func TestArtifactStoreCreatesPrivateRootOrRejectsUnsafeExistingRoots(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	t.Run("creates absent root privately", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "artifacts")
		store, err := NewArtifactStore(root, key, NewRedactor(nil))
		if err != nil {
			t.Fatalf("NewArtifactStore(): %v", err)
		}
		artifacts, err := store.Begin("attempt-07")
		if err != nil {
			t.Fatalf("Begin(): %v", err)
		}
		if _, err := artifacts.Finalize("sha256:plan", "sha256:baseline", "sha256:final"); err != nil {
			t.Fatalf("Finalize(): %v", err)
		}
		info, err := os.Stat(root)
		if err != nil {
			t.Fatalf("stat artifact root: %v", err)
		}
		if info.Mode().Perm() != 0o700 {
			t.Fatalf("artifact root permissions = %o, want 700", info.Mode().Perm())
		}
	})

	for _, test := range []struct {
		name  string
		setup func(t *testing.T, root string)
	}{
		{
			name: "symlink",
			setup: func(t *testing.T, root string) {
				target := filepath.Join(t.TempDir(), "target")
				if err := os.Mkdir(target, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, root); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "non-directory",
			setup: func(t *testing.T, root string) {
				if err := os.WriteFile(root, []byte("not a directory"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "insecure permissions",
			setup: func(t *testing.T, root string) {
				if err := os.Mkdir(root, 0o755); err != nil {
					t.Fatal(err)
				}
			},
		},
	} {
		t.Run("rejects "+test.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "artifacts")
			test.setup(t, root)
			store, err := NewArtifactStore(root, key, NewRedactor(nil))
			if err != nil {
				t.Fatalf("NewArtifactStore(): %v", err)
			}
			if _, err := store.Begin("attempt-08"); err == nil {
				t.Fatal("Begin() accepted an unsafe existing artifact root")
			}
			info, err := os.Lstat(root)
			if err != nil {
				t.Fatalf("lstat artifact root: %v", err)
			}
			if test.name == "insecure permissions" && info.Mode().Perm() != 0o755 {
				t.Fatalf("artifact root permissions = %o, want unchanged 755", info.Mode().Perm())
			}
		})
	}
}

// TestAttemptArtifactsRejectsNonMonotonicEvents keeps retained timelines unambiguous.
func TestAttemptArtifactsRejectsNonMonotonicEvents(t *testing.T) {
	store, err := NewArtifactStore(filepath.Join(t.TempDir(), "artifacts"), []byte("0123456789abcdef0123456789abcdef"), NewRedactor(nil))
	if err != nil {
		t.Fatalf("NewArtifactStore(): %v", err)
	}
	artifacts, err := store.Begin("attempt-04")
	if err != nil {
		t.Fatalf("Begin(): %v", err)
	}
	if err := artifacts.AppendEvent(Event{Sequence: 2, Kind: EventMessage}); err != nil {
		t.Fatalf("AppendEvent(): %v", err)
	}
	if err := artifacts.AppendEvent(Event{Sequence: 2, Kind: EventMessage}); err == nil || !strings.Contains(err.Error(), "must be greater") {
		t.Fatalf("AppendEvent() error = %v, want sequence rejection", err)
	}
	if _, err := artifacts.Finalize("sha256:plan", "sha256:baseline", "sha256:final"); err != nil {
		t.Fatalf("Finalize(): %v", err)
	}
}

// TestVerifyArtifactManifestDetectsTampering proves retained evidence cannot change silently.
func TestVerifyArtifactManifestDetectsTampering(t *testing.T) {
	root := filepath.Join(t.TempDir(), "artifacts")
	key := []byte("0123456789abcdef0123456789abcdef")
	store, err := NewArtifactStore(root, key, NewRedactor(nil))
	if err != nil {
		t.Fatalf("NewArtifactStore(): %v", err)
	}
	artifacts, err := store.Begin("attempt-03")
	if err != nil {
		t.Fatalf("Begin(): %v", err)
	}
	if err := artifacts.WriteText("summary.txt", "original"); err != nil {
		t.Fatalf("WriteText(): %v", err)
	}
	if _, err := artifacts.Finalize("sha256:plan", "sha256:baseline", "sha256:final"); err != nil {
		t.Fatalf("Finalize(): %v", err)
	}
	directory := filepath.Join(root, "attempt-03")
	if err := os.WriteFile(filepath.Join(directory, "summary.txt"), []byte("tampered"), 0o600); err != nil {
		t.Fatalf("tamper with artifact: %v", err)
	}
	if _, err := VerifyArtifactManifest(directory, key); err == nil || !strings.Contains(err.Error(), "do not match") {
		t.Fatalf("VerifyArtifactManifest() error = %v, want tamper detection", err)
	}
}

// TestReadVerifiedAttemptSummaryReturnsOnlyAuthenticatedContent keeps report rendering behind the manifest boundary.
func TestReadVerifiedAttemptSummaryReturnsOnlyAuthenticatedContent(t *testing.T) {
	root := filepath.Join(t.TempDir(), "artifacts")
	key := []byte("0123456789abcdef0123456789abcdef")
	store, err := NewArtifactStore(root, key, NewRedactor(nil))
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := store.Begin("attempt-report")
	if err != nil {
		t.Fatal(err)
	}
	if err := artifacts.WriteText("summary.txt", "diagnostic summary\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := artifacts.Finalize("sha256:plan", "sha256:baseline", "sha256:final"); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(root, "attempt-report")
	summary, manifest, err := ReadVerifiedAttemptSummary(directory, key)
	if err != nil {
		t.Fatalf("ReadVerifiedAttemptSummary(): %v", err)
	}
	if summary != "diagnostic summary\n" || manifest.AttemptID != "attempt-report" {
		t.Fatalf("summary = %q, manifest = %#v", summary, manifest)
	}
	if err := os.WriteFile(filepath.Join(directory, "summary.txt"), []byte("tampered\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ReadVerifiedAttemptSummary(directory, key); err == nil {
		t.Fatal("ReadVerifiedAttemptSummary() accepted a tampered summary")
	}
}

// TestVerifyArtifactManifestRejectsNonCanonicalJSON prevents parser differentials from authenticating altered raw evidence.
func TestVerifyArtifactManifestRejectsNonCanonicalJSON(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	for _, test := range []struct {
		name   string
		mutate func(string) string
		want   string
	}{
		{
			name: "unknown field",
			mutate: func(body string) string {
				return strings.Replace(body, "\n}", ",\n  \"untrusted\": true\n}", 1)
			},
			want: "unknown field",
		},
		{
			name: "duplicate field",
			mutate: func(body string) string {
				return strings.Replace(body, "{\n  \"schema_version\": 1,", "{\n  \"schema_version\": 1,\n  \"schema_version\": 1,", 1)
			},
			want: "canonical form",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			directory := finalizedArtifactDirectory(t, key)
			path := filepath.Join(directory, "manifest.json")
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(test.mutate(string(body))), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := VerifyArtifactManifest(directory, key); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("VerifyArtifactManifest() error = %v, want %q rejection", err, test.want)
			}
		})
	}
}

// TestVerifyArtifactManifestRejectsUnsafeManifestFiles bounds authentication before reading untrusted file content.
func TestVerifyArtifactManifestRejectsUnsafeManifestFiles(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	t.Run("symlink", func(t *testing.T) {
		directory := finalizedArtifactDirectory(t, key)
		path := filepath.Join(directory, "manifest.json")
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(directory, "summary.txt"), path); err != nil {
			t.Fatal(err)
		}
		if _, err := VerifyArtifactManifest(directory, key); err == nil || !strings.Contains(err.Error(), "regular file") {
			t.Fatalf("VerifyArtifactManifest() error = %v, want symlink rejection", err)
		}
	})
	t.Run("oversized", func(t *testing.T) {
		directory := finalizedArtifactDirectory(t, key)
		path := filepath.Join(directory, "manifest.json")
		if err := os.WriteFile(path, bytes.Repeat([]byte{'x'}, maxArtifactManifestSize+1), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := VerifyArtifactManifest(directory, key); err == nil || !strings.Contains(err.Error(), "exceeds") {
			t.Fatalf("VerifyArtifactManifest() error = %v, want size rejection", err)
		}
	})
	t.Run("directory", func(t *testing.T) {
		directory := finalizedArtifactDirectory(t, key)
		path := filepath.Join(directory, "manifest.json")
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		if _, err := VerifyArtifactManifest(directory, key); err == nil || !strings.Contains(err.Error(), "regular file") {
			t.Fatalf("VerifyArtifactManifest() error = %v, want special-file rejection", err)
		}
	})
}

// TestVerifyArtifactManifestRejectsEntryOverflow bounds directory enumeration to the fixed artifact surface.
func TestVerifyArtifactManifestRejectsEntryOverflow(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	directory := finalizedArtifactDirectory(t, key)
	for index := 0; index < maxArtifactDirectoryEntries; index++ {
		name := filepath.Join(directory, fmt.Sprintf("overflow-%02d", index))
		if err := os.WriteFile(name, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want := fmt.Sprintf("exceeds %d entries", maxArtifactDirectoryEntries)
	if _, err := VerifyArtifactManifest(directory, key); err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("VerifyArtifactManifest() error = %v, want bounded-entry rejection", err)
	}
}

// TestArtifactOperationsRejectUnknownFiles keeps finalization and verification on the declared evidence surface.
func TestArtifactOperationsRejectUnknownFiles(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	t.Run("finalize", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "artifacts")
		store, err := NewArtifactStore(root, key, NewRedactor(nil))
		if err != nil {
			t.Fatal(err)
		}
		artifacts, err := store.Begin("unknown-finalize")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(artifacts.directory, "unknown.txt"), nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := artifacts.Finalize("sha256:plan", "sha256:baseline", "sha256:final"); err == nil || !strings.Contains(err.Error(), "allowed artifact surface") {
			t.Fatalf("Finalize() error = %v, want unknown-file rejection", err)
		}
	})
	t.Run("verify", func(t *testing.T) {
		directory := finalizedArtifactDirectory(t, key)
		if err := os.WriteFile(filepath.Join(directory, "unknown.txt"), nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := VerifyArtifactManifest(directory, key); err == nil || !strings.Contains(err.Error(), "allowed artifact surface") {
			t.Fatalf("VerifyArtifactManifest() error = %v, want unknown-file rejection", err)
		}
	})
}

// finalizedArtifactDirectory creates one authenticated fixture whose manifest can be adversarially replaced.
func finalizedArtifactDirectory(t *testing.T, key []byte) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "artifacts")
	store, err := NewArtifactStore(root, key, NewRedactor(nil))
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := store.Begin("manifest-fixture")
	if err != nil {
		t.Fatal(err)
	}
	if err := artifacts.WriteText("summary.txt", "fixture"); err != nil {
		t.Fatal(err)
	}
	if _, err := artifacts.Finalize("sha256:plan", "sha256:baseline", "sha256:final"); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(root, "manifest-fixture")
}

// TestNewArtifactStoreRequiresAuthenticationKey prevents accidentally unsigned diagnostic evidence.
func TestNewArtifactStoreRequiresAuthenticationKey(t *testing.T) {
	if _, err := NewArtifactStore(t.TempDir(), []byte("short"), NewRedactor(nil)); err == nil {
		t.Fatal("NewArtifactStore() accepted a short authentication key")
	}
}

// TestVerifyArtifactManifestRequiresAuthenticationKey rejects verification keys that cannot safely authenticate evidence.
func TestVerifyArtifactManifestRequiresAuthenticationKey(t *testing.T) {
	for _, key := range [][]byte{nil, []byte("short"), []byte("0123456789abcdef0123456789abc")} {
		if _, err := VerifyArtifactManifest(t.TempDir(), key); err == nil || !strings.Contains(err.Error(), "at least 32 bytes") {
			t.Fatalf("VerifyArtifactManifest() error = %v, want authentication key rejection", err)
		}
	}
}

// TestArtifactStoreCanaryRejectsRegisteredSecretsBeforeFinalization keeps bypassed writes from becoming authenticated evidence.
func TestArtifactStoreCanaryRejectsRegisteredSecretsBeforeFinalization(t *testing.T) {
	root := filepath.Join(t.TempDir(), "artifacts")
	secret := "canary-secret-not-for-artifacts"
	store, err := NewArtifactStore(root, []byte("0123456789abcdef0123456789abcdef"), NewRedactor([]string{secret}))
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := store.Begin("attempt-05")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "attempt-05", "summary.txt"), []byte(secret), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := artifacts.Finalize("sha256:plan", "sha256:baseline", "sha256:final"); err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("Finalize() error = %v, want secret-safe canary rejection", err)
	}
	if _, err := os.Stat(filepath.Join(root, "attempt-05", "manifest.json")); !os.IsNotExist(err) {
		t.Fatalf("manifest after canary rejection = %v, want absent", err)
	}
}

// TestArtifactStoreRedactsQuotedCredentialTokens keeps Codex credential values out of all persisted diagnostic surfaces.
func TestArtifactStoreRedactsQuotedCredentialTokens(t *testing.T) {
	accessToken := "quoted-access-token-value"
	token := "quoted-token-value"
	store, err := NewArtifactStore(filepath.Join(t.TempDir(), "artifacts"), []byte("0123456789abcdef0123456789abcdef"), NewRedactor([]string{accessToken, token}))
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := store.Begin("attempt-06")
	if err != nil {
		t.Fatal(err)
	}
	quotedCredential := `{"access_token":"` + accessToken + `","token":"` + token + `"}`
	if err := artifacts.AppendEvent(Event{Sequence: 1, Kind: EventMessage, Fields: map[string]string{"credential": quotedCredential}}); err != nil {
		t.Fatal(err)
	}
	if err := artifacts.WriteText("transcript.redacted.txt", quotedCredential); err != nil {
		t.Fatal(err)
	}
	if err := artifacts.WriteText("diff.patch", `+credential `+quotedCredential); err != nil {
		t.Fatal(err)
	}
	if _, err := artifacts.Finalize("sha256:plan", "sha256:baseline", "sha256:final"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"events.jsonl", "transcript.redacted.txt", "diff.patch"} {
		body, err := os.ReadFile(filepath.Join(artifacts.directory, name))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(body), accessToken) || strings.Contains(string(body), token) {
			t.Fatalf("%s retained a credential token: %q", name, body)
		}
	}
}

// TestRedactorRemovesProviderPrefixedAssignments keeps environment-style credential names from bypassing generic redaction.
func TestRedactorRemovesProviderPrefixedAssignments(t *testing.T) {
	redacted := NewRedactor(nil).Text("OPENAI_API_KEY=provider-secret")
	if strings.Contains(redacted, "provider-secret") || !strings.Contains(redacted, redactedValue) {
		t.Fatalf("prefixed credential assignment survived redaction: %q", redacted)
	}
}
