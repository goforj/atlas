package eval

import (
	"path/filepath"
	"testing"
)

// TestPrivateVerifierEnvironmentTreatsNilAndEmptyAsClean prevents candidate initialization from inheriting supervisor credentials.
func TestPrivateVerifierEnvironmentTreatsNilAndEmptyAsClean(t *testing.T) {
	t.Setenv("AWS_SECRET_ACCESS_KEY", "ambient-aws-secret")
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "/private/google-credentials.json")
	t.Setenv("GITHUB_TOKEN", "ambient-github-token")
	t.Setenv("SSH_AUTH_SOCK", "/private/ssh-agent.sock")
	for _, test := range []struct {
		name string
		base []string
	}{
		{name: "nil", base: nil},
		{name: "empty", base: []string{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			stateRoot := filepath.Join(t.TempDir(), "state")
			environment, err := privateVerifierEnvironment(test.base, stateRoot, "")
			if err != nil {
				t.Fatalf("privateVerifierEnvironment(): %v", err)
			}
			values := verifierEnvironmentValues(environment)
			for _, name := range []string{"AWS_SECRET_ACCESS_KEY", "GOOGLE_APPLICATION_CREDENTIALS", "GITHUB_TOKEN", "SSH_AUTH_SOCK"} {
				if value, inherited := values[name]; inherited {
					t.Fatalf("ambient %s reached candidate initialization as %q", name, value)
				}
			}
			if values["HOME"] != filepath.Join(stateRoot, "home") || values["GOWORK"] != "off" {
				t.Fatalf("private verifier environment = %#v", values)
			}
		})
	}
}

// TestPrivateVerifierEnvironmentPreservesExplicitAllowlist keeps host integrations in control of non-secret tool settings.
func TestPrivateVerifierEnvironmentPreservesExplicitAllowlist(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	environment, err := privateVerifierEnvironment([]string{"PATH=/trusted/tools", "LANG=en_US.UTF-8"}, stateRoot, "")
	if err != nil {
		t.Fatalf("privateVerifierEnvironment(): %v", err)
	}
	values := verifierEnvironmentValues(environment)
	if values["PATH"] != "/trusted/tools" || values["LANG"] != "en_US.UTF-8" {
		t.Fatalf("explicit verifier allowlist was not preserved: %#v", values)
	}
}
