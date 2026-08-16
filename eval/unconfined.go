package eval

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// UnconfinedLocal provides disposable process state without claiming host, command, filesystem, or network isolation.
type UnconfinedLocal struct {
	WorkRoot string
}

// Name returns the backend identity recorded in diagnostic artifacts.
func (UnconfinedLocal) Name() string {
	return "unconfined-local"
}

// Capabilities intentionally returns none because host-local process execution is not authoritative evidence.
func (UnconfinedLocal) Capabilities(context.Context) ([]Capability, error) {
	return nil, nil
}

// Open creates private agent state outside the candidate-writable Project tree.
func (backend UnconfinedLocal) Open(_ context.Context, request BackendRequest) (BackendEnvironment, error) {
	if request.Project == nil {
		return nil, fmt.Errorf("prepared Project is required")
	}
	projectRoot, err := filepath.Abs(request.Project.Result().ProjectRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve Project root: %w", err)
	}
	if strings.TrimSpace(projectRoot) == "" {
		return nil, fmt.Errorf("prepared Project root is required")
	}
	homeRoot, err := os.MkdirTemp(backend.WorkRoot, "atlas-eval-home-")
	if err != nil {
		return nil, fmt.Errorf("create private agent home: %w", err)
	}
	return &unconfinedEnvironment{
		environment: RunEnvironment{
			ProjectRoot: projectRoot,
			HomeRoot:    homeRoot,
			Environment: append([]string(nil), request.Environment...),
		},
	}, nil
}

// unconfinedEnvironment owns the disposable agent home used by one diagnostic attempt.
type unconfinedEnvironment struct {
	environment RunEnvironment
	sealedRoot  string
	mu          sync.Mutex
	closeOnce   sync.Once
	closeErr    error
}

// Environment returns the private paths without attaching capabilities the backend cannot prove.
func (environment *unconfinedEnvironment) Environment() RunEnvironment {
	return environment.environment
}

// Baseline reports the diagnostic baseline without claiming the complete supervisor provenance required for authoritative evaluation.
func (environment *unconfinedEnvironment) Baseline(context.Context) (BaselineSnapshot, error) {
	if environment == nil {
		return BaselineSnapshot{}, fmt.Errorf("unconfined environment is required")
	}
	digest, err := digestProjectTree(environment.environment.ProjectRoot)
	if err != nil {
		return BaselineSnapshot{}, err
	}
	return BaselineSnapshot{TreeDigest: digest}, nil
}

// ObservedEvents returns no action evidence because unconfined execution has no supervisor monitor.
func (*unconfinedEnvironment) ObservedEvents(context.Context) ([]Event, error) {
	return nil, nil
}

// Seal copies the stopped candidate tree into backend-owned verifier input and records its deterministic digest.
func (environment *unconfinedEnvironment) Seal(_ context.Context) (SealedProject, error) {
	if environment == nil {
		return SealedProject{}, fmt.Errorf("unconfined environment is required")
	}
	environment.mu.Lock()
	defer environment.mu.Unlock()
	if environment.sealedRoot != "" {
		digest, err := digestProjectTree(environment.sealedRoot)
		return SealedProject{Root: environment.sealedRoot, TreeDigest: digest}, err
	}
	root, err := os.MkdirTemp(filepath.Dir(environment.environment.HomeRoot), "atlas-eval-sealed-")
	if err != nil {
		return SealedProject{}, fmt.Errorf("create sealed Project: %w", err)
	}
	if err := copyProjectTree(environment.environment.ProjectRoot, root); err != nil {
		return SealedProject{}, errors.Join(err, os.RemoveAll(root))
	}
	digest, err := digestProjectTree(root)
	if err != nil {
		return SealedProject{}, errors.Join(err, os.RemoveAll(root))
	}
	environment.sealedRoot = root
	return SealedProject{Root: root, TreeDigest: digest}, nil
}

// Close removes private agent state while leaving Project cleanup to the preparer owner.
func (environment *unconfinedEnvironment) Close(context.Context) error {
	if environment == nil {
		return nil
	}
	environment.closeOnce.Do(func() {
		environment.mu.Lock()
		sealedRoot := environment.sealedRoot
		environment.mu.Unlock()
		environment.closeErr = errors.Join(os.RemoveAll(environment.environment.HomeRoot), os.RemoveAll(sealedRoot))
	})
	return environment.closeErr
}

// digestProjectTree binds relative paths, modes, symlink targets, and regular file content in lexical order.
func digestProjectTree(root string) (string, error) {
	var paths []string
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path != root {
			paths = append(paths, path)
		}
		return nil
	}); err != nil {
		return "", err
	}
	sort.Strings(paths)
	hash := sha256.New()
	for _, path := range paths {
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return "", err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(hash, "%s\x00%s\x00", filepath.ToSlash(relative), info.Mode().String())
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return "", err
			}
			fmt.Fprintf(hash, "%s\x00", target)
			continue
		}
		if !info.Mode().IsRegular() {
			continue
		}
		file, err := os.Open(path)
		if err != nil {
			return "", err
		}
		_, copyErr := io.Copy(hash, file)
		closeErr := file.Close()
		if err := errors.Join(copyErr, closeErr); err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("sha256:%x", hash.Sum(nil)), nil
}
