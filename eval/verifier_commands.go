package eval

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/goforj/atlas/internal/processgroup"
)

const (
	// verifierCommandTimeout leaves headroom for concurrent private cold-cache builds without permitting an indefinite child process.
	verifierCommandTimeout = 4 * time.Minute
	maxVerifierOutput      = 1 << 20
	maxVerifierReadFile    = 4 << 20
	maxProjectTreeEntries  = 25_000
	maxProjectTreeBytes    = int64(2 << 30)
)

// VerifierCommands executes an allowlisted toolchain in a disposable copy of the candidate Project.
type VerifierCommands struct {
	WorkRoot       string
	GoExecutable   string
	ForjExecutable string
	Environment    []string
	// ModuleProxy is the host-owned read-only Go module proxy exposed to verifier commands.
	ModuleProxy string
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

// FilesNamed lists regular candidate files by basename without allowing the caller to traverse outside the disposable Project.
func (session *verifierCommandSession) FilesNamed(name string) ([]string, error) {
	if session == nil {
		return nil, fmt.Errorf("verifier command session is required")
	}
	if filepath.Base(name) != name || name == "." || name == "" {
		return nil, fmt.Errorf("verifier filename %q must be a basename", name)
	}
	var matches []string
	err := filepath.WalkDir(session.root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && path != session.root {
			switch entry.Name() {
			case ".git", "node_modules":
				return filepath.SkipDir
			}
		}
		if entry.Name() != name || entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		relative, err := filepath.Rel(session.root, path)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return fmt.Errorf("verifier file %q escapes the disposable Project", path)
		}
		matches = append(matches, filepath.ToSlash(relative))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list verifier files named %q: %w", name, err)
	}
	slices.Sort(matches)
	return matches, nil
}

// ReadFile returns one regular candidate file from the disposable Project.
func (session *verifierCommandSession) ReadFile(relativePath string) ([]byte, error) {
	if session == nil {
		return nil, fmt.Errorf("verifier command session is required")
	}
	if !filepath.IsLocal(relativePath) || filepath.Clean(relativePath) == "." {
		return nil, fmt.Errorf("verifier file path %q must remain relative to the disposable Project", relativePath)
	}
	path := filepath.Join(session.root, filepath.Clean(relativePath))
	within, err := filepath.Rel(session.root, path)
	if err != nil || within == ".." || strings.HasPrefix(within, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("verifier file path %q escapes the disposable Project", relativePath)
	}
	parent := filepath.Dir(path)
	parentWithin, err := filepath.Rel(session.root, parent)
	if err != nil || parentWithin == ".." || strings.HasPrefix(parentWithin, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("verifier file path %q escapes the disposable Project", relativePath)
	}
	current := session.root
	if parentWithin != "." {
		for _, component := range strings.Split(parentWithin, string(filepath.Separator)) {
			current = filepath.Join(current, component)
			parentInfo, parentErr := os.Lstat(current)
			if parentErr != nil {
				return nil, fmt.Errorf("inspect verifier file parent %q: %w", relativePath, parentErr)
			}
			if !parentInfo.IsDir() || parentInfo.Mode()&os.ModeSymlink != 0 {
				return nil, fmt.Errorf("verifier file parent %q must contain only existing directories", relativePath)
			}
		}
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("inspect verifier file %q: %w", relativePath, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("verifier file %q must be a regular file", relativePath)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open verifier file %q: %w", relativePath, err)
	}
	opened, statErr := file.Stat()
	if statErr != nil || !os.SameFile(info, opened) {
		_ = file.Close()
		return nil, fmt.Errorf("verifier file %q changed while it was opened", relativePath)
	}
	body, readErr := io.ReadAll(io.LimitReader(file, maxVerifierReadFile+1))
	closeErr := file.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return nil, fmt.Errorf("read verifier file %q: %w", relativePath, err)
	}
	if len(body) > maxVerifierReadFile {
		return nil, fmt.Errorf("verifier file %q exceeds %d bytes", relativePath, maxVerifierReadFile)
	}
	return body, nil
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
func (runner VerifierCommands) Open(ctx context.Context, project VerifierProject) (CommandSession, error) {
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
	if err := copyVerifierProjectTree(ctx, project, root); err != nil {
		return nil, errors.Join(err, removeVerifierOwnedTree(root))
	}
	stateRoot, err := os.MkdirTemp(runner.WorkRoot, "atlas-verifier-state-")
	if err != nil {
		return nil, errors.Join(fmt.Errorf("create verifier state: %w", err), removeVerifierOwnedTree(root))
	}
	environment, err := privateVerifierEnvironment(runner.Environment, stateRoot, runner.ModuleProxy)
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
func privateVerifierEnvironment(base []string, stateRoot, moduleProxy string) ([]string, error) {
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
	moduleProxy = strings.TrimSpace(moduleProxy)
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

// verifierEnvironmentValue returns the final value for a named entry in an explicit process environment.
func verifierEnvironmentValue(environment []string, name string) string {
	for index := len(environment) - 1; index >= 0; index-- {
		key, value, found := strings.Cut(environment[index], "=")
		if found && key == name {
			return value
		}
	}
	return ""
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
	ctx, cancel := verifierCommandContext(ctx)
	defer cancel()
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
	cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), verifierCleanupTimeout)
	terminateErr := process.Terminate(cleanupContext)
	cleanupCancel()
	err = errors.Join(err, terminateErr)
	if output.exceeded() {
		err = errors.Join(err, fmt.Errorf("verifier command output exceeds %d bytes", maxVerifierOutput))
	}
	if err != nil {
		return output.String(), fmt.Errorf("%s: %w\n%s", strings.Join(command, " "), err, strings.TrimSpace(output.String()))
	}
	return output.String(), nil
}

// verifierCommandContext applies the command ceiling while preserving an earlier evaluation deadline.
func verifierCommandContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, verifierCommandTimeout)
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

// copyVerifierProjectTree excludes candidate tests and restores only tests captured before the agent received the Project.
func copyVerifierProjectTree(ctx context.Context, project VerifierProject, destinationRoot string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := copyProjectTreeWithFilter(ctx, project.Root, destinationRoot, func(relative string) bool {
		return strings.HasSuffix(relative, "_test.go")
	}); err != nil {
		return err
	}
	if len(project.BaselineTests) > maxProjectTreeEntries {
		return fmt.Errorf("trusted tests exceed %d entries", maxProjectTreeEntries)
	}
	var trustedBytes int64
	for _, test := range project.BaselineTests {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !filepath.IsLocal(test.Path) || !strings.HasSuffix(filepath.ToSlash(test.Path), "_test.go") {
			return fmt.Errorf("trusted test path %q is invalid", test.Path)
		}
		if baselineTestExcluded(filepath.ToSlash(test.Path), project.BaselineTestExclusions) {
			continue
		}
		trustedBytes += int64(len(test.Body))
		if trustedBytes > maxTrustedTestRetainedContentSize {
			return fmt.Errorf("trusted tests exceed %d bytes", maxTrustedTestRetainedContentSize)
		}
		destination := filepath.Join(destinationRoot, filepath.FromSlash(test.Path))
		if err := prepareTrustedTestParent(destinationRoot, filepath.Dir(destination)); err != nil {
			return fmt.Errorf("prepare trusted test %q: %w", test.Path, err)
		}
		file, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, os.FileMode(test.Mode).Perm())
		if err != nil {
			return fmt.Errorf("restore trusted test %q: %w", test.Path, err)
		}
		_, writeErr := file.Write(test.Body)
		if err := errors.Join(writeErr, file.Close()); err != nil {
			return fmt.Errorf("restore trusted test %q: %w", test.Path, err)
		}
	}
	return nil
}

// baselineTestExcluded limits compatibility exclusions to explicitly reviewed baseline paths.
func baselineTestExcluded(path string, exclusions []string) bool {
	for _, exclusion := range exclusions {
		matched, err := filepath.Match(exclusion, path)
		if err == nil && matched {
			return true
		}
	}
	return false
}

// prepareTrustedTestParent recreates deleted baseline directories without following candidate-controlled symlinks.
func prepareTrustedTestParent(root, parent string) error {
	relative, err := filepath.Rel(root, parent)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("trusted test parent escapes the disposable Project")
	}
	current := root
	if relative == "." {
		return nil
	}
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			if err := os.Mkdir(current, 0o755); err != nil {
				return fmt.Errorf("create directory %q: %w", component, err)
			}
			continue
		}
		if err != nil {
			return fmt.Errorf("inspect directory %q: %w", component, err)
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("directory %q must not be replaced by a symlink or file", component)
		}
	}
	return nil
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
