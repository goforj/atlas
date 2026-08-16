//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd

package eval

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestArtifactOperationsRejectFIFOs ensures special files cannot enter either artifact operation.
func TestArtifactOperationsRejectFIFOs(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	t.Run("finalize", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "artifacts")
		store, err := NewArtifactStore(root, key, NewRedactor(nil))
		if err != nil {
			t.Fatal(err)
		}
		artifacts, err := store.Begin("fifo-finalize")
		if err != nil {
			t.Fatal(err)
		}
		if err := syscall.Mkfifo(filepath.Join(artifacts.directory, "summary.txt"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := artifacts.Finalize("sha256:plan", "sha256:baseline", "sha256:final"); err == nil || !strings.Contains(err.Error(), "regular file") {
			t.Fatalf("Finalize() error = %v, want FIFO rejection", err)
		}
	})
	t.Run("verify", func(t *testing.T) {
		directory := finalizedArtifactDirectory(t, key)
		path := filepath.Join(directory, "summary.txt")
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := syscall.Mkfifo(path, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := VerifyArtifactManifest(directory, key); err == nil || !strings.Contains(err.Error(), "regular file") {
			t.Fatalf("VerifyArtifactManifest() error = %v, want FIFO rejection", err)
		}
	})
}

// TestOpenArtifactFileRejectsFIFOSwapWithoutBlocking exercises the race between enumeration and descriptor open.
func TestOpenArtifactFileRejectsFIFOSwapWithoutBlocking(t *testing.T) {
	directoryPath := t.TempDir()
	path := filepath.Join(directoryPath, "summary.txt")
	if err := os.WriteFile(path, []byte("regular"), 0o600); err != nil {
		t.Fatal(err)
	}
	directory, err := openArtifactDirectory(directoryPath)
	if err != nil {
		t.Fatal(err)
	}
	defer directory.close()
	pathInfo := directory.entries["summary.txt"]
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() {
		file, err := openArtifactFile(directory.root, "summary.txt", pathInfo)
		if file != nil {
			_ = file.Close()
		}
		result <- err
	}()
	select {
	case err := <-result:
		if err == nil || !strings.Contains(err.Error(), "changed while opening") {
			t.Fatalf("openArtifactFile() error = %v, want swapped-file rejection", err)
		}
	case <-time.After(time.Second):
		t.Fatal("openArtifactFile() blocked after a FIFO swap")
	}
}
