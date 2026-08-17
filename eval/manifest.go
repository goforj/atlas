package eval

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// promotedEvaluationFiles contains only reviewed manifests and prompts shipped with Atlas.
//
//go:embed evaluations/*/evaluation.yaml evaluations/*/prompt.md
var promotedEvaluationFiles embed.FS

var promotedEvaluationDirectories = map[string]string{
	"add-http-controller": "evaluations/add_http_controller",
}

const evaluationManifestSchemaVersion = 1

var evaluationIDPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
var contractIDPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*(?:/[a-z0-9]+(?:[.-][a-z0-9]+)*)$`)

// evaluationManifestV1 is the closed join contract between a prompt and promoted IDs.
type evaluationManifestV1 struct {
	SchemaVersion   int                `yaml:"schema_version"`
	ID              string             `yaml:"id"`
	Summary         string             `yaml:"summary"`
	Suite           string             `yaml:"suite"`
	ProjectScenario string             `yaml:"project_scenario"`
	Workflow        string             `yaml:"workflow"`
	Verifier        string             `yaml:"verifier"`
	Limits          evaluationLimitsV1 `yaml:"limits"`
}

// evaluationLimitsV1 keeps human-friendly duration syntax at the wire boundary.
type evaluationLimitsV1 struct {
	WallTime     string `yaml:"wall_time"`
	Commands     int    `yaml:"commands"`
	ShellNetwork string `yaml:"shell_network"`
}

// LoadDefinition reads one strict evaluation manifest and its adjacent prompt.
func LoadDefinition(directory string) (EvaluationDefinition, error) {
	manifestPath := filepath.Join(directory, "evaluation.yaml")
	body, err := os.ReadFile(manifestPath)
	if err != nil {
		return EvaluationDefinition{}, fmt.Errorf("read evaluation manifest: %w", err)
	}
	prompt, err := os.ReadFile(filepath.Join(directory, "prompt.md"))
	if err != nil {
		return EvaluationDefinition{}, fmt.Errorf("read evaluation prompt: %w", err)
	}
	return loadDefinitionContent(body, prompt)
}

// LoadPromotedDefinition reads one reviewed evaluation bundled with Atlas.
func LoadPromotedDefinition(id string) (EvaluationDefinition, error) {
	directory, ok := promotedEvaluationDirectories[id]
	if !ok {
		return EvaluationDefinition{}, fmt.Errorf("evaluation %q is not promoted", id)
	}
	body, err := promotedEvaluationFiles.ReadFile(directory + "/evaluation.yaml")
	if err != nil {
		return EvaluationDefinition{}, fmt.Errorf("read promoted evaluation manifest: %w", err)
	}
	prompt, err := promotedEvaluationFiles.ReadFile(directory + "/prompt.md")
	if err != nil {
		return EvaluationDefinition{}, fmt.Errorf("read promoted evaluation prompt: %w", err)
	}
	return loadDefinitionContent(body, prompt)
}

// loadDefinitionContent joins a strict manifest with its exact adjacent prompt bytes.
func loadDefinitionContent(body, prompt []byte) (EvaluationDefinition, error) {
	manifest, err := decodeEvaluationManifest(body)
	if err != nil {
		return EvaluationDefinition{}, fmt.Errorf("evaluation.yaml: %w", err)
	}
	if strings.TrimSpace(string(prompt)) == "" {
		return EvaluationDefinition{}, fmt.Errorf("prompt.md is empty")
	}
	digest := sha256.Sum256(prompt)
	return EvaluationDefinition{
		SchemaVersion:   manifest.SchemaVersion,
		ID:              manifest.ID,
		Summary:         manifest.Summary,
		Suite:           manifest.Suite,
		ProjectScenario: manifest.ProjectScenario,
		WorkflowID:      manifest.WorkflowID,
		VerifierID:      manifest.VerifierID,
		Limits:          manifest.Limits,
		Prompt:          string(prompt),
		PromptDigest:    fmt.Sprintf("sha256:%x", digest),
	}, nil
}

// decodeEvaluationManifest rejects ambiguity before resolving any promoted contract.
func decodeEvaluationManifest(body []byte) (EvaluationDefinition, error) {
	if err := rejectEvaluationYAMLFeatures(body); err != nil {
		return EvaluationDefinition{}, err
	}
	var wire evaluationManifestV1
	decoder := yaml.NewDecoder(bytes.NewReader(body))
	decoder.KnownFields(true)
	if err := decoder.Decode(&wire); err != nil {
		return EvaluationDefinition{}, err
	}
	var trailing yaml.Node
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return EvaluationDefinition{}, fmt.Errorf("multiple YAML documents are not supported")
		}
		return EvaluationDefinition{}, fmt.Errorf("decode trailing YAML: %w", err)
	}
	if wire.SchemaVersion != evaluationManifestSchemaVersion {
		return EvaluationDefinition{}, fmt.Errorf("unsupported schema_version %d", wire.SchemaVersion)
	}
	if !evaluationIDPattern.MatchString(wire.ID) {
		return EvaluationDefinition{}, fmt.Errorf("id %q must be a safe slug", wire.ID)
	}
	if strings.TrimSpace(wire.Summary) == "" {
		return EvaluationDefinition{}, fmt.Errorf("summary is required")
	}
	if !evaluationIDPattern.MatchString(wire.Suite) {
		return EvaluationDefinition{}, fmt.Errorf("suite %q must be a safe slug", wire.Suite)
	}
	if !evaluationIDPattern.MatchString(wire.ProjectScenario) {
		return EvaluationDefinition{}, fmt.Errorf("project_scenario %q must be a safe slug", wire.ProjectScenario)
	}
	if !contractIDPattern.MatchString(wire.Workflow) {
		return EvaluationDefinition{}, fmt.Errorf("workflow %q must be a versioned contract ID", wire.Workflow)
	}
	if !contractIDPattern.MatchString(wire.Verifier) {
		return EvaluationDefinition{}, fmt.Errorf("verifier %q must be a versioned contract ID", wire.Verifier)
	}
	wallTime, err := time.ParseDuration(wire.Limits.WallTime)
	if err != nil || wallTime <= 0 {
		return EvaluationDefinition{}, fmt.Errorf("limits.wall_time %q must be a positive duration", wire.Limits.WallTime)
	}
	if wire.Limits.Commands <= 0 {
		return EvaluationDefinition{}, fmt.Errorf("limits.commands must be positive")
	}
	if wire.Limits.ShellNetwork != "off" && wire.Limits.ShellNetwork != "fixture-only" {
		return EvaluationDefinition{}, fmt.Errorf("limits.shell_network %q is unsupported", wire.Limits.ShellNetwork)
	}
	return EvaluationDefinition{
		SchemaVersion:   wire.SchemaVersion,
		ID:              wire.ID,
		Summary:         wire.Summary,
		Suite:           wire.Suite,
		ProjectScenario: wire.ProjectScenario,
		WorkflowID:      wire.Workflow,
		VerifierID:      wire.Verifier,
		Limits: Limits{
			WallTime:     wallTime,
			Commands:     wire.Limits.Commands,
			ShellNetwork: wire.Limits.ShellNetwork,
		},
	}, nil
}

// rejectEvaluationYAMLFeatures prevents hidden aliases, anchors, and merge semantics.
func rejectEvaluationYAMLFeatures(body []byte) error {
	var document yaml.Node
	if err := yaml.Unmarshal(body, &document); err != nil {
		return err
	}
	var inspect func(*yaml.Node) error
	inspect = func(node *yaml.Node) error {
		if node.Kind == yaml.AliasNode || node.Anchor != "" {
			return fmt.Errorf("YAML aliases and anchors are not supported")
		}
		if node.Kind == yaml.MappingNode {
			for index := 0; index < len(node.Content); index += 2 {
				if node.Content[index].Value == "<<" {
					return fmt.Errorf("YAML merge keys are not supported")
				}
			}
		}
		for _, child := range node.Content {
			if err := inspect(child); err != nil {
				return err
			}
		}
		return nil
	}
	return inspect(&document)
}
