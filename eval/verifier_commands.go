package eval

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/goforj/atlas/internal/processgroup"
)

const (
	// verifierCommandTimeout allows a generated Project to compile from a private cold build cache without permitting an indefinite child process.
	verifierCommandTimeout = 2 * time.Minute
	maxVerifierOutput      = 1 << 20
	maxProjectTreeEntries  = 25_000
	maxProjectTreeBytes    = int64(2 << 30)
)

// VerifierCommands executes an allowlisted toolchain in a disposable copy of the candidate Project.
type VerifierCommands struct {
	WorkRoot       string
	GoExecutable   string
	ForjExecutable string
	Environment    []string
}

// verifierCommandSession owns one private candidate clone and its allowlisted command identities.
type verifierCommandSession struct {
	root           string
	stateRoot      string
	goExecutable   string
	forjExecutable string
	environment    []string
	closeOnce      sync.Once
	closeErr       error
}

// WriteFile installs supervisor-owned verifier source without allowing candidate paths to escape the disposable clone.
func (session *verifierCommandSession) WriteFile(relativePath string, body []byte) error {
	if session == nil {
		return fmt.Errorf("verifier command session is required")
	}
	if !filepath.IsLocal(relativePath) || filepath.Clean(relativePath) == "." {
		return fmt.Errorf("verifier file path %q must remain relative to the disposable Project", relativePath)
	}
	destination := filepath.Join(session.root, filepath.Clean(relativePath))
	parent := filepath.Dir(destination)
	within, err := filepath.Rel(session.root, parent)
	if err != nil || within == ".." || strings.HasPrefix(within, ".."+string(filepath.Separator)) {
		return fmt.Errorf("verifier file path %q escapes the disposable Project", relativePath)
	}
	current := session.root
	if within != "." {
		for _, component := range strings.Split(within, string(filepath.Separator)) {
			current = filepath.Join(current, component)
			info, err := os.Lstat(current)
			if err != nil {
				return fmt.Errorf("inspect verifier file parent %q: %w", relativePath, err)
			}
			if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("verifier file parent %q must contain only existing directories", relativePath)
			}
		}
	}
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create verifier file %q: %w", relativePath, err)
	}
	_, writeErr := file.Write(body)
	return errors.Join(writeErr, file.Close())
}

// Open clones the sealed Project for exactly one verifier phase, so no phase can observe another phase's candidate-controlled mutation.
func (runner VerifierCommands) Open(ctx context.Context, sourceRoot string) (CommandSession, error) {
	goExecutable, err := resolveVerifierExecutable(runner.GoExecutable, "go")
	if err != nil {
		return nil, err
	}
	forjExecutable, err := resolveVerifierExecutable(runner.ForjExecutable, "forj")
	if err != nil {
		return nil, err
	}
	root, err := os.MkdirTemp(runner.WorkRoot, "atlas-verifier-project-")
	if err != nil {
		return nil, fmt.Errorf("create verifier Project: %w", err)
	}
	if err := copyVerifierProjectTree(ctx, sourceRoot, root); err != nil {
		return nil, errors.Join(err, removeVerifierOwnedTree(root))
	}
	stateRoot, err := os.MkdirTemp(runner.WorkRoot, "atlas-verifier-state-")
	if err != nil {
		return nil, errors.Join(fmt.Errorf("create verifier state: %w", err), removeVerifierOwnedTree(root))
	}
	environment, err := privateVerifierEnvironment(runner.Environment, stateRoot)
	if err != nil {
		return nil, errors.Join(err, removeVerifierOwnedTree(root), removeVerifierOwnedTree(stateRoot))
	}
	return &verifierCommandSession{
		root:           root,
		stateRoot:      stateRoot,
		goExecutable:   goExecutable,
		forjExecutable: forjExecutable,
		environment:    environment,
	}, nil
}

// privateVerifierEnvironment gives one verifier phase no reusable writable state from another phase or the supervisor.
func privateVerifierEnvironment(base []string, stateRoot string) ([]string, error) {
	paths := map[string]string{
		"HOME":            filepath.Join(stateRoot, "home"),
		"GOCACHE":         filepath.Join(stateRoot, "go-cache"),
		"GOMODCACHE":      filepath.Join(stateRoot, "go-mod-cache"),
		"GOPATH":          filepath.Join(stateRoot, "go-path"),
		"GOTMPDIR":        filepath.Join(stateRoot, "tmp"),
		"TMPDIR":          filepath.Join(stateRoot, "tmp"),
		"TEMP":            filepath.Join(stateRoot, "tmp"),
		"TMP":             filepath.Join(stateRoot, "tmp"),
		"XDG_CACHE_HOME":  filepath.Join(stateRoot, "xdg-cache"),
		"XDG_CONFIG_HOME": filepath.Join(stateRoot, "xdg-config"),
	}
	for _, path := range paths {
		if err := os.MkdirAll(path, 0o700); err != nil {
			return nil, fmt.Errorf("create private verifier state: %w", err)
		}
	}
	moduleProxy := verifierModuleProxy(base)
	environment := make([]string, 0, len(base)+len(paths)+1)
	for _, entry := range base {
		name, _, found := strings.Cut(entry, "=")
		if !found {
			continue
		}
		if _, overridden := paths[name]; overridden || name == "GOWORK" || (moduleProxy != "" && name == "GOPROXY") {
			continue
		}
		environment = append(environment, entry)
	}
	for _, name := range []string{"HOME", "GOCACHE", "GOMODCACHE", "GOPATH", "GOTMPDIR", "TMPDIR", "TEMP", "TMP", "XDG_CACHE_HOME", "XDG_CONFIG_HOME"} {
		environment = append(environment, name+"="+paths[name])
	}
	if moduleProxy != "" {
		environment = append(environment, "GOPROXY="+moduleProxy)
	}
	environment = append(environment, "GOWORK=off")
	return environment, nil
}

// verifierModuleProxy reuses only immutable module archives from trusted preparation while keeping every verifier's writable module state private.
func verifierModuleProxy(environment []string) string {
	values := verifierEnvironmentMap(environment)
	moduleCache := strings.TrimSpace(values["GOMODCACHE"])
	if moduleCache == "" {
		return ""
	}
	downloadRoot := filepath.Join(moduleCache, "cache", "download")
	info, err := os.Stat(downloadRoot)
	if err != nil || !info.IsDir() {
		return ""
	}
	proxyPath := filepath.ToSlash(downloadRoot)
	if filepath.VolumeName(downloadRoot) != "" && !strings.HasPrefix(proxyPath, "/") {
		proxyPath = "/" + proxyPath
	}
	local := (&url.URL{Scheme: "file", Path: proxyPath}).String()
	upstream := strings.TrimSpace(values["GOPROXY"])
	if upstream == "" || upstream == "off" {
		return local
	}
	return local + "," + upstream
}

// verifierEnvironmentMap resolves the final value of each process environment entry.
func verifierEnvironmentMap(environment []string) map[string]string {
	values := make(map[string]string, len(environment))
	for _, entry := range environment {
		name, value, found := strings.Cut(entry, "=")
		if found {
			values[name] = value
		}
	}
	return values
}

// Run executes one deadline- and output-bounded allowlisted process group, then cleans up its group and any descendants observed on supported hosts.
func (session *verifierCommandSession) Run(ctx context.Context, command []string) (string, error) {
	if len(command) == 0 {
		return "", fmt.Errorf("verifier command is required")
	}
	executable := ""
	switch command[0] {
	case "go":
		executable = session.goExecutable
	case "forj":
		executable = session.forjExecutable
	default:
		return "", fmt.Errorf("verifier command %q is not allowlisted", command[0])
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, verifierCommandTimeout)
		defer cancel()
	}
	output := &boundedVerifierOutput{limit: maxVerifierOutput}
	process, err := processgroup.Start(executable, command[1:], processgroup.Options{
		Dir:    session.root,
		Env:    append([]string(nil), session.environment...),
		Stdout: output,
		Stderr: output,
	})
	if err != nil {
		return output.String(), fmt.Errorf("start %s: %w", strings.Join(command, " "), err)
	}
	err = process.Wait(ctx)
	// A candidate may have backgrounded descendants. They share this dedicated
	// group and must not survive into a later verifier phase.
	cleanupContext, cancel := context.WithTimeout(context.Background(), verifierCleanupTimeout)
	terminateErr := process.Terminate(cleanupContext)
	cancel()
	err = errors.Join(err, terminateErr)
	if output.exceeded() {
		err = errors.Join(err, fmt.Errorf("verifier command output exceeds %d bytes", maxVerifierOutput))
	}
	if err != nil {
		return output.String(), fmt.Errorf("%s: %w\n%s", strings.Join(command, " "), err, strings.TrimSpace(output.String()))
	}
	return output.String(), nil
}

// boundedVerifierOutput prevents untrusted candidate output from consuming supervisor memory.
type boundedVerifierOutput struct {
	mu       sync.Mutex
	buffer   bytes.Buffer
	limit    int
	overflow bool
}

// Write retains a bounded prefix and tells os/exec to close the pipe once the limit is reached.
func (output *boundedVerifierOutput) Write(body []byte) (int, error) {
	output.mu.Lock()
	defer output.mu.Unlock()
	remaining := output.limit - output.buffer.Len()
	if remaining > 0 {
		if len(body) > remaining {
			output.buffer.Write(body[:remaining])
			output.overflow = true
			return remaining, errors.New("verifier output limit reached")
		}
		output.buffer.Write(body)
		return len(body), nil
	}
	output.overflow = true
	return 0, errors.New("verifier output limit reached")
}

// String returns the retained diagnostic prefix.
func (output *boundedVerifierOutput) String() string {
	output.mu.Lock()
	defer output.mu.Unlock()
	return output.buffer.String()
}

// exceeded reports whether output was truncated.
func (output *boundedVerifierOutput) exceeded() bool {
	output.mu.Lock()
	defer output.mu.Unlock()
	return output.overflow
}

// Close destroys the verifier's mutable Project clone without touching sealed candidate evidence.
func (session *verifierCommandSession) Close(context.Context) error {
	if session == nil {
		return nil
	}
	session.closeOnce.Do(func() {
		session.closeErr = errors.Join(removeVerifierOwnedTree(session.root), removeVerifierOwnedTree(session.stateRoot))
	})
	return session.closeErr
}

// removeVerifierOwnedTree restores traversal only inside disposable verifier state before removing read-only module caches.
func removeVerifierOwnedTree(root string) error {
	if _, err := os.Lstat(root); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	walkErr := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return os.Chmod(path, 0o700)
		}
		return nil
	})
	return errors.Join(walkErr, os.RemoveAll(root))
}

// resolveVerifierExecutable pins each allowlisted command before any candidate source executes.
func resolveVerifierExecutable(candidate, fallback string) (string, error) {
	if strings.TrimSpace(candidate) == "" {
		candidate = fallback
	}
	path, err := exec.LookPath(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve verifier executable %q: %w", candidate, err)
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve verifier executable %q: %w", candidate, err)
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("verifier executable %q is not a regular file", path)
	}
	return path, nil
}

// copyProjectTree preserves every supported candidate path in a bounded, cancellation-aware snapshot.
func copyProjectTree(ctx context.Context, sourceRoot, destinationRoot string) error {
	return copyProjectTreeWithFilter(ctx, sourceRoot, destinationRoot, func(string) bool { return false })
}

// copyVerifierProjectTree excludes candidate tests because verifier phases execute only supervisor-authored tests.
func copyVerifierProjectTree(ctx context.Context, sourceRoot, destinationRoot string) error {
	return copyProjectTreeWithFilter(ctx, sourceRoot, destinationRoot, func(relative string) bool {
		return strings.HasSuffix(relative, "_test.go")
	})
}

// copyProjectTreeWithFilter preserves regular files, directories, modes, and safe internal symlinks in a private clone.
func copyProjectTreeWithFilter(ctx context.Context, sourceRoot, destinationRoot string, skipFile func(string) bool) error {
	if ctx == nil {
		ctx = context.Background()
	}
	sourceRoot, err := filepath.Abs(sourceRoot)
	if err != nil {
		return fmt.Errorf("resolve source Project: %w", err)
	}
	entries := 0
	remainingBytes := maxProjectTreeBytes
	return filepath.WalkDir(sourceRoot, func(sourcePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		relative, err := filepath.Rel(sourceRoot, sourcePath)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		entries++
		if entries > maxProjectTreeEntries {
			return fmt.Errorf("Project tree exceeds %d entries", maxProjectTreeEntries)
		}
		destinationPath := filepath.Join(destinationRoot, relative)
		info, err := os.Lstat(sourcePath)
		if err != nil {
			return err
		}
		switch {
		case info.IsDir():
			return os.Mkdir(destinationPath, info.Mode().Perm())
		case info.Mode().IsRegular():
			if skipFile(relative) {
				return nil
			}
			if info.Size() < 0 || info.Size() > remainingBytes {
				return fmt.Errorf("Project tree exceeds %d bytes", maxProjectTreeBytes)
			}
			remainingBytes -= info.Size()
			return copyRegularFile(ctx, sourcePath, destinationPath, info.Mode().Perm(), info.Size())
		case info.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(sourcePath)
			if err != nil {
				return err
			}
			resolved := target
			if !filepath.IsAbs(resolved) {
				resolved = filepath.Join(filepath.Dir(sourcePath), resolved)
			}
			resolved, err = filepath.Abs(resolved)
			if err != nil {
				return err
			}
			within, err := filepath.Rel(sourceRoot, resolved)
			if err != nil || within == ".." || strings.HasPrefix(within, ".."+string(filepath.Separator)) {
				return fmt.Errorf("Project symlink %q escapes the candidate root", relative)
			}
			return os.Symlink(target, destinationPath)
		default:
			return fmt.Errorf("unsupported Project file type at %q", relative)
		}
	})
}

// copyRegularFile copies one immutable source into verifier-owned storage without following symlinks.
func copyRegularFile(ctx context.Context, source, destination string, mode os.FileMode, maxBytes int64) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		_ = input.Close()
		return err
	}
	buffer := make([]byte, 32<<10)
	var copied int64
	var copyErr error
	for copyErr == nil {
		if err := ctx.Err(); err != nil {
			copyErr = err
			break
		}
		read, readErr := input.Read(buffer)
		if read > 0 {
			copied += int64(read)
			if copied > maxBytes {
				copyErr = fmt.Errorf("Project file %q changed size while copying", source)
				break
			}
			written, writeErr := output.Write(buffer[:read])
			if writeErr != nil {
				copyErr = writeErr
				break
			}
			if written != read {
				copyErr = io.ErrShortWrite
				break
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			copyErr = readErr
		}
	}
	if copyErr == nil && copied != maxBytes {
		copyErr = fmt.Errorf("Project file %q changed size while copying", source)
	}
	closeErr := errors.Join(input.Close(), output.Close())
	if copyErr != nil {
		return errors.Join(copyErr, closeErr, os.Remove(destination))
	}
	return closeErr
}
