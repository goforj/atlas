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
	"strings"
	"sync"
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

// Open clones the sealed Project once so verifier commands can share generated build state without mutating candidate evidence.
func (runner VerifierCommands) Open(_ context.Context, sourceRoot string) (CommandSession, error) {
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
	if err := copyProjectTree(sourceRoot, root); err != nil {
		return nil, errors.Join(err, os.RemoveAll(root))
	}
	return &verifierCommandSession{
		root:           root,
		goExecutable:   goExecutable,
		forjExecutable: forjExecutable,
		environment:    append([]string(nil), runner.Environment...),
	}, nil
}

// Run executes only the declared Go or GoForj tool identity and captures bounded process output through the caller's context.
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
	cmd := exec.CommandContext(ctx, executable, command[1:]...)
	cmd.Dir = session.root
	cmd.Env = append([]string(nil), session.environment...)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		return output.String(), fmt.Errorf("%s: %w\n%s", strings.Join(command, " "), err, strings.TrimSpace(output.String()))
	}
	return output.String(), nil
}

// Close destroys the verifier's mutable Project clone without touching sealed candidate evidence.
func (session *verifierCommandSession) Close(context.Context) error {
	if session == nil {
		return nil
	}
	session.closeOnce.Do(func() {
		session.closeErr = os.RemoveAll(session.root)
	})
	return session.closeErr
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

// copyProjectTree preserves regular files, directories, modes, and safe internal symlinks in the verifier-owned clone.
func copyProjectTree(sourceRoot, destinationRoot string) error {
	sourceRoot, err := filepath.Abs(sourceRoot)
	if err != nil {
		return fmt.Errorf("resolve source Project: %w", err)
	}
	return filepath.WalkDir(sourceRoot, func(sourcePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(sourceRoot, sourcePath)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
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
			return copyRegularFile(sourcePath, destinationPath, info.Mode().Perm())
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
func copyRegularFile(source, destination string, mode os.FileMode) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		_ = input.Close()
		return err
	}
	_, copyErr := io.Copy(output, input)
	return errors.Join(copyErr, input.Close(), output.Close())
}
