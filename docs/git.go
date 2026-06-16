package docs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
)

// DefaultRepo is the canonical GoForj docs repository Atlas reads from.
const DefaultRepo = "https://github.com/goforj/docs.git"

// DefaultRef is the docs branch Atlas tracks when no explicit ref is configured.
const DefaultRef = "main"

// EnvCacheDir overrides the local cache used for cloned docs.
const EnvCacheDir = "GOFORJ_ATLAS_DOCS_CACHE"

// GitProvider loads GoForj docs from a local git cache.
type GitProvider struct {
	CacheDir string
	Repo     string
	Ref      string
	Version  string
	Refresh  bool

	mu       sync.Mutex
	synced   bool
	root     string
	revision string
	err      error
	git      gitClient
}

var errGitUnavailable = errors.New("git executable unavailable")

type gitClient interface {
	Clone(context.Context, string, string) error
	Fetch(context.Context, string) error
	Checkout(context.Context, string, string) error
	Pull(context.Context, string, string) error
	Revision(context.Context, string) (string, error)
}

// NewGitProvider returns a git-backed docs provider with normal GoForj defaults.
func NewGitProvider(version string) *GitProvider {
	return &GitProvider{
		Repo:    DefaultRepo,
		Ref:     DefaultRef,
		Version: version,
		Refresh: true,
	}
}

// Manifest returns docs metadata from the cached checkout.
func (p *GitProvider) Manifest(ctx context.Context) (Manifest, error) {
	root, revision, err := p.ensure(ctx)
	if err != nil {
		return Manifest{}, err
	}
	return FSProvider{Root: root, Version: p.Version, Revision: revision}.Manifest(ctx)
}

// Documents returns Markdown documents from the cached checkout.
func (p *GitProvider) Documents(ctx context.Context) ([]Document, error) {
	root, revision, err := p.ensure(ctx)
	if err != nil {
		return nil, err
	}
	return FSProvider{Root: root, Version: p.Version, Revision: revision}.Documents(ctx)
}

// ensure synchronizes the git cache once per provider instance.
func (p *GitProvider) ensure(ctx context.Context) (string, string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.synced {
		return p.root, p.revision, p.err
	}

	p.root, p.revision, p.err = p.sync(ctx)
	p.synced = true
	return p.root, p.revision, p.err
}

// sync prefers fresh docs but keeps an existing cache usable when refresh fails.
func (p *GitProvider) sync(ctx context.Context) (string, string, error) {
	cacheDir, err := p.cacheDir()
	if err != nil {
		return "", "", err
	}
	repo := firstNonEmpty(p.Repo, DefaultRepo)
	ref := firstNonEmpty(p.Ref, DefaultRef)
	client := p.gitClient()

	if !dirExists(filepath.Join(cacheDir, ".git")) {
		if err := os.MkdirAll(filepath.Dir(cacheDir), 0o755); err != nil {
			return "", "", err
		}
		if err := client.Clone(ctx, repo, cacheDir); err != nil {
			return "", "", err
		}
	}

	if p.Refresh {
		if err := client.Fetch(ctx, cacheDir); err != nil && !hasUsableDocs(cacheDir) {
			return "", "", err
		}
	}
	if ref != "" {
		if err := client.Checkout(ctx, cacheDir, ref); err != nil && !hasUsableDocs(cacheDir) {
			return "", "", err
		}
		if ref == DefaultRef {
			if err := client.Pull(ctx, cacheDir, ref); err != nil && !hasUsableDocs(cacheDir) {
				return "", "", err
			}
		}
	}

	revision, err := client.Revision(ctx, cacheDir)
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(revision) == "" {
		return "", "", errors.New("docs git cache has no revision")
	}
	return cacheDir, revision, nil
}

// cacheDir resolves the docs cache without requiring each GoForj project to own a checkout.
func (p *GitProvider) cacheDir() (string, error) {
	if strings.TrimSpace(p.CacheDir) != "" {
		return filepath.Abs(p.CacheDir)
	}
	if value := strings.TrimSpace(os.Getenv(EnvCacheDir)); value != "" {
		return filepath.Abs(value)
	}
	cacheRoot, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheRoot, "goforj", "atlas", "docs"), nil
}

// hasUsableDocs lets offline refresh attempts continue when a previous cache exists.
func hasUsableDocs(root string) bool {
	return dirExists(docsRoot(root))
}

// gitClient returns a fast shell-backed client with a native fallback when git is absent.
func (p *GitProvider) gitClient() gitClient {
	if p.git != nil {
		return p.git
	}
	return fallbackGitClient{
		shell:  shellGitClient{},
		native: nativeGitClient{},
	}
}

type fallbackGitClient struct {
	shell  gitClient
	native gitClient
}

func (c fallbackGitClient) Clone(ctx context.Context, repo string, path string) error {
	err := c.shell.Clone(ctx, repo, path)
	if errors.Is(err, errGitUnavailable) {
		return c.native.Clone(ctx, repo, path)
	}
	return err
}

func (c fallbackGitClient) Fetch(ctx context.Context, path string) error {
	err := c.shell.Fetch(ctx, path)
	if errors.Is(err, errGitUnavailable) {
		return c.native.Fetch(ctx, path)
	}
	return err
}

func (c fallbackGitClient) Checkout(ctx context.Context, path string, ref string) error {
	err := c.shell.Checkout(ctx, path, ref)
	if errors.Is(err, errGitUnavailable) {
		return c.native.Checkout(ctx, path, ref)
	}
	return err
}

func (c fallbackGitClient) Pull(ctx context.Context, path string, ref string) error {
	err := c.shell.Pull(ctx, path, ref)
	if errors.Is(err, errGitUnavailable) {
		return c.native.Pull(ctx, path, ref)
	}
	return err
}

func (c fallbackGitClient) Revision(ctx context.Context, path string) (string, error) {
	revision, err := c.shell.Revision(ctx, path)
	if errors.Is(err, errGitUnavailable) {
		return c.native.Revision(ctx, path)
	}
	return revision, err
}

type shellGitClient struct{}

func (shellGitClient) Clone(ctx context.Context, repo string, path string) error {
	return runShellGit(ctx, "", "clone", repo, path)
}

func (shellGitClient) Fetch(ctx context.Context, path string) error {
	return runShellGit(ctx, path, "fetch", "--all", "--tags", "--prune")
}

func (shellGitClient) Checkout(ctx context.Context, path string, ref string) error {
	return runShellGit(ctx, path, "checkout", ref)
}

func (shellGitClient) Pull(ctx context.Context, path string, _ string) error {
	return runShellGit(ctx, path, "pull", "--ff-only")
}

func (shellGitClient) Revision(ctx context.Context, path string) (string, error) {
	out, err := runShellGitOutput(ctx, path, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// runShellGit wraps git errors with stderr so refresh failures are actionable.
func runShellGit(ctx context.Context, dir string, args ...string) error {
	_, err := runShellGitOutput(ctx, dir, args...)
	return err
}

// runShellGitOutput marks missing shell git separately so native fallback can take over.
func runShellGitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		var execErr *exec.Error
		if errors.As(err, &execErr) && errors.Is(execErr.Err, exec.ErrNotFound) {
			return "", fmt.Errorf("%w", errGitUnavailable)
		}
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

type nativeGitClient struct{}

func (nativeGitClient) Clone(ctx context.Context, repo string, path string) error {
	_, err := git.PlainCloneContext(ctx, path, false, &git.CloneOptions{URL: repo})
	return err
}

func (nativeGitClient) Fetch(ctx context.Context, path string) error {
	repository, err := git.PlainOpen(path)
	if err != nil {
		return err
	}
	err = repository.FetchContext(ctx, &git.FetchOptions{
		RemoteName: "origin",
		RefSpecs: []config.RefSpec{
			"+refs/heads/*:refs/remotes/origin/*",
			"+refs/tags/*:refs/tags/*",
		},
		Tags:  git.AllTags,
		Force: true,
	})
	if errors.Is(err, git.NoErrAlreadyUpToDate) {
		return nil
	}
	return err
}

func (c nativeGitClient) Checkout(_ context.Context, path string, ref string) error {
	repository, err := git.PlainOpen(path)
	if err != nil {
		return err
	}
	worktree, err := repository.Worktree()
	if err != nil {
		return err
	}
	for _, name := range referenceCandidates(ref) {
		reference, err := repository.Reference(name, true)
		if err != nil {
			continue
		}
		return worktree.Checkout(&git.CheckoutOptions{Hash: reference.Hash()})
	}
	if plumbing.IsHash(ref) {
		return worktree.Checkout(&git.CheckoutOptions{Hash: plumbing.NewHash(ref)})
	}
	return fmt.Errorf("git ref not found: %s", ref)
}

func (c nativeGitClient) Pull(ctx context.Context, path string, ref string) error {
	if err := c.Fetch(ctx, path); err != nil {
		return err
	}
	return c.Checkout(ctx, path, ref)
}

func (nativeGitClient) Revision(_ context.Context, path string) (string, error) {
	repository, err := git.PlainOpen(path)
	if err != nil {
		return "", err
	}
	head, err := repository.Head()
	if err != nil {
		return "", err
	}
	return head.Hash().String(), nil
}

// referenceCandidates supports branch, remote branch, and tag refs without user flags.
func referenceCandidates(ref string) []plumbing.ReferenceName {
	return []plumbing.ReferenceName{
		plumbing.NewBranchReferenceName(ref),
		plumbing.NewRemoteReferenceName("origin", ref),
		plumbing.NewTagReferenceName(ref),
		plumbing.ReferenceName(ref),
	}
}

// firstNonEmpty keeps defaults local without pulling in a broader helper package.
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
