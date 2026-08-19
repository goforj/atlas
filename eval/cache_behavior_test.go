package eval

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestResolveCacheBehaviorProbeAcceptsConventionalDecoratorFamilies keeps cache behavior independent of repository type names.
func TestResolveCacheBehaviorProbeAcceptsConventionalDecoratorFamilies(t *testing.T) {
	for _, test := range []struct {
		name        string
		source      string
		want        string
		constructor string
	}{
		{
			name: "source repository decorator",
			source: `package users
type User struct{}
type Repository interface { Find() }
type SourceRepository struct{}
type CachedRepository struct { source Repository; cache *Cache }
func NewRepository(source Repository, cache *Cache) Repository { return &CachedRepository{source: source, cache: cache} }
func (*CachedRepository) Find() {}
`,
			want:        "&CachedRepository{source: source, cache: profileCache}",
			constructor: "NewRepository",
		},
		{
			name: "user repository decorator",
			source: `package users
type User struct{}
type UserRepository interface { Find() }
type CachedUserRepository struct { next UserRepository; profiles *ProfileCache }
func NewCachedUserRepository(next UserRepository, profiles *ProfileCache) UserRepository { return &CachedUserRepository{next: next, profiles: profiles} }
func (*CachedUserRepository) Find() {}
`,
			want:        "&CachedUserRepository{next: source, profiles: profileCache}",
			constructor: "NewCachedUserRepository",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			directory := filepath.Join(root, "internal", "users")
			if err := os.MkdirAll(directory, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(directory, "repository.go"), []byte(test.source), 0o600); err != nil {
				t.Fatal(err)
			}
			probe, err := resolveCacheBehaviorProbe(root)
			if err != nil {
				t.Fatalf("resolveCacheBehaviorProbe(): %v", err)
			}
			body, err := renderCacheBehaviorProbe(probe)
			if err != nil {
				t.Fatalf("renderCacheBehaviorProbe(): %v", err)
			}
			if !strings.Contains(string(body), test.want) {
				t.Fatalf("probe does not construct discovered decorator:\n%s", body)
			}
			if probe.constructor != test.constructor {
				t.Fatalf("constructor = %q, want %q", probe.constructor, test.constructor)
			}
		})
	}
}

// TestVerifyCacheDecoratorRegistrationRequiresTheDiscoveredConstructor keeps Wire evidence tied to the selected decorator.
func TestVerifyCacheDecoratorRegistrationRequiresTheDiscoveredConstructor(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "app", "wire")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "inject_services_app.go")
	if err := os.WriteFile(path, []byte("package wire\nimport \"github.com/google/wire\"\nvar provider = wire.NewSet(users.NewRepository)\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyCacheDecoratorRegistration(root, "NewRepository"); err != nil {
		t.Fatalf("verifyCacheDecoratorRegistration(): %v", err)
	}
	if err := verifyCacheDecoratorRegistration(root, "NewCachedRepository"); err == nil {
		t.Fatal("missing discovered constructor registration passed")
	}
}

// TestVerifyCacheDecoratorRegistrationAcceptsRegisteredWrapper requires the
// wrapper itself to be in the Wire set, rather than accepting an unused helper.
func TestVerifyCacheDecoratorRegistrationAcceptsRegisteredWrapper(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "app", "wire")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "inject_services_app.go")
	registered := "package wire\nimport \"github.com/google/wire\"\nfunc provideRepository() any { return users.NewCachedRepository(nil, nil) }\nvar provider = wire.NewSet(provideRepository)\n"
	if err := os.WriteFile(path, []byte(registered), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyCacheDecoratorRegistration(root, "NewCachedRepository"); err != nil {
		t.Fatalf("registered wrapper: %v", err)
	}
	unregistered := "package wire\nimport \"github.com/google/wire\"\nfunc provideRepository() any { return users.NewCachedRepository(nil, nil) }\nfunc other() any { return nil }\nvar provider = wire.NewSet(other)\n"
	if err := os.WriteFile(path, []byte(unregistered), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyCacheDecoratorRegistration(root, "NewCachedRepository"); err == nil {
		t.Fatal("unregistered wrapper passed")
	}
}

// TestVerifyCacheDecoratorRegistrationAcceptsPackageWrapper preserves a registered composition constructor around the testable decorator.
func TestVerifyCacheDecoratorRegistrationAcceptsPackageWrapper(t *testing.T) {
	root := t.TempDir()
	wireDirectory := filepath.Join(root, "app", "wire")
	usersDirectory := filepath.Join(root, "internal", "users")
	if err := os.MkdirAll(wireDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(usersDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	wireSource := "package wire\nimport (\n\t\"github.com/google/wire\"\n\t\"example.com/project/internal/users\"\n)\nvar provider = wire.NewSet(users.NewRepository)\n"
	if err := os.WriteFile(filepath.Join(wireDirectory, "inject_services_app.go"), []byte(wireSource), 0o600); err != nil {
		t.Fatal(err)
	}
	usersSource := "package users\nfunc NewRepository() any { return NewCachedRepository(nil, nil) }\nfunc NewCachedRepository(any, any) any { return nil }\n"
	if err := os.WriteFile(filepath.Join(usersDirectory, "repository.go"), []byte(usersSource), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyCacheDecoratorRegistration(root, "NewCachedRepository"); err != nil {
		t.Fatalf("registered package wrapper: %v", err)
	}
	usersSource = "package users\nfunc NewRepository() any { return nil }\nfunc NewCachedRepository(any, any) any { return nil }\n"
	if err := os.WriteFile(filepath.Join(usersDirectory, "repository.go"), []byte(usersSource), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyCacheDecoratorRegistration(root, "NewCachedRepository"); err == nil {
		t.Fatal("package wrapper disconnected from decorator passed")
	}
}

// TestResolveCacheBehaviorProbeFallsBackToTwoDependencyConstructor supports decorators that do not expose fields.
func TestResolveCacheBehaviorProbeFallsBackToTwoDependencyConstructor(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "internal", "users")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	source := `package users
type User struct{}
type Repository interface { Find() }
type Cache struct{}
func NewCachedRepository(repository Repository, cache *Cache) Repository { return repository }
`
	if err := os.WriteFile(filepath.Join(directory, "repository.go"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	probe, err := resolveCacheBehaviorProbe(root)
	if err != nil {
		t.Fatalf("resolveCacheBehaviorProbe(): %v", err)
	}
	body, err := renderCacheBehaviorProbe(probe)
	if err != nil {
		t.Fatalf("renderCacheBehaviorProbe(): %v", err)
	}
	if !strings.Contains(string(body), "NewCachedRepository(source, profileCache)") {
		t.Fatalf("probe does not use two-dependency constructor:\n%s", body)
	}
}
