package eval

import (
	_ "embed"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const coverageCatalogSchemaVersion = 1

var capabilityIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]*(?:\.[a-z][a-z0-9-]*)+$`)

//go:embed coverage.yaml
var promotedCoverageCatalog []byte

// CoverageTier identifies how frequently a capability should participate in live evaluation.
type CoverageTier string

const (
	// CoverageTierSmoke identifies release-critical behavior suitable for every evaluation smoke run.
	CoverageTierSmoke CoverageTier = "smoke"
	// CoverageTierCore identifies representative framework behavior for the complete core benchmark.
	CoverageTierCore CoverageTier = "core"
	// CoverageTierExtended identifies expensive or specialized behavior for scheduled qualification.
	CoverageTierExtended CoverageTier = "extended"
)

// CapabilityCoverage maps one durable framework capability to the evaluations that currently measure it.
type CapabilityCoverage struct {
	ID          string       `json:"id"`
	Area        string       `json:"area"`
	Summary     string       `json:"summary"`
	Tier        CoverageTier `json:"tier"`
	Evaluations []string     `json:"evaluations"`
}

// CoverageCatalog is the validated framework capability inventory used to expose measured and planned coverage.
type CoverageCatalog struct {
	Capabilities []CapabilityCoverage `json:"capabilities"`
}

// Covered reports whether at least one promoted evaluation currently measures the capability.
func (coverage CapabilityCoverage) Covered() bool {
	return len(coverage.Evaluations) > 0
}

// EvaluationIDs returns the promoted evaluations needed to exercise capabilities through the requested tier.
func (catalog CoverageCatalog) EvaluationIDs(tier CoverageTier) ([]string, error) {
	limit, ok := coverageTierRank(tier)
	if !ok {
		return nil, fmt.Errorf("unknown evaluation coverage tier %q", tier)
	}
	selected := map[string]bool{}
	for _, capability := range catalog.Capabilities {
		rank, valid := coverageTierRank(capability.Tier)
		if !valid || rank > limit {
			continue
		}
		for _, evaluationID := range capability.Evaluations {
			selected[evaluationID] = true
		}
	}
	ids := make([]string, 0, len(selected))
	for evaluationID := range selected {
		ids = append(ids, evaluationID)
	}
	sort.Strings(ids)
	return ids, nil
}

// coverageTierRank keeps tier selection cumulative so core includes every release-critical smoke evaluation.
func coverageTierRank(tier CoverageTier) (int, bool) {
	switch tier {
	case CoverageTierSmoke:
		return 0, true
	case CoverageTierCore:
		return 1, true
	case CoverageTierExtended:
		return 2, true
	default:
		return 0, false
	}
}

// LoadCoverageCatalog returns a validated copy of the promoted framework capability inventory.
func LoadCoverageCatalog() (CoverageCatalog, error) {
	var wire coverageCatalogWire
	decoder := yaml.NewDecoder(strings.NewReader(string(promotedCoverageCatalog)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&wire); err != nil {
		return CoverageCatalog{}, fmt.Errorf("decode evaluation coverage catalog: %w", err)
	}
	if wire.SchemaVersion != coverageCatalogSchemaVersion {
		return CoverageCatalog{}, fmt.Errorf("unsupported evaluation coverage schema %d", wire.SchemaVersion)
	}
	coreIDs, err := PromotedEvaluationIDs("core")
	if err != nil {
		return CoverageCatalog{}, err
	}
	allIDs, err := PromotedEvaluationIDs("")
	if err != nil {
		return CoverageCatalog{}, err
	}
	knownEvaluations := make(map[string]bool, len(allIDs))
	for _, id := range allIDs {
		knownEvaluations[id] = true
	}
	capabilities := make([]CapabilityCoverage, 0, len(wire.Capabilities))
	seenCapabilities := map[string]bool{}
	coveredCore := map[string]bool{}
	previousID := ""
	for _, entry := range wire.Capabilities {
		coverage, err := validateCapabilityCoverage(entry, knownEvaluations)
		if err != nil {
			return CoverageCatalog{}, err
		}
		if seenCapabilities[coverage.ID] {
			return CoverageCatalog{}, fmt.Errorf("duplicate evaluation capability %q", coverage.ID)
		}
		if previousID != "" && coverage.ID < previousID {
			return CoverageCatalog{}, fmt.Errorf("evaluation capabilities are not sorted: %q appears after %q", coverage.ID, previousID)
		}
		previousID = coverage.ID
		seenCapabilities[coverage.ID] = true
		for _, evaluationID := range coverage.Evaluations {
			coveredCore[evaluationID] = true
		}
		capabilities = append(capabilities, coverage)
	}
	for _, evaluationID := range coreIDs {
		if !coveredCore[evaluationID] {
			return CoverageCatalog{}, fmt.Errorf("core evaluation %q is absent from the capability catalog", evaluationID)
		}
	}
	return CoverageCatalog{Capabilities: capabilities}, nil
}

// validateCapabilityCoverage rejects catalog drift before reports can present misleading coverage.
func validateCapabilityCoverage(entry capabilityCoverageWire, knownEvaluations map[string]bool) (CapabilityCoverage, error) {
	if !capabilityIDPattern.MatchString(entry.ID) {
		return CapabilityCoverage{}, fmt.Errorf("evaluation capability ID %q is invalid", entry.ID)
	}
	if !evaluationIDPattern.MatchString(entry.Area) {
		return CapabilityCoverage{}, fmt.Errorf("evaluation capability %q has invalid area %q", entry.ID, entry.Area)
	}
	if strings.TrimSpace(entry.Summary) == "" {
		return CapabilityCoverage{}, fmt.Errorf("evaluation capability %q requires a summary", entry.ID)
	}
	if entry.Tier != CoverageTierSmoke && entry.Tier != CoverageTierCore && entry.Tier != CoverageTierExtended {
		return CapabilityCoverage{}, fmt.Errorf("evaluation capability %q has invalid tier %q", entry.ID, entry.Tier)
	}
	evaluations := append([]string(nil), entry.Evaluations...)
	if !sort.StringsAreSorted(evaluations) {
		return CapabilityCoverage{}, fmt.Errorf("evaluation capability %q references evaluations out of order", entry.ID)
	}
	for index, evaluationID := range evaluations {
		if !knownEvaluations[evaluationID] {
			return CapabilityCoverage{}, fmt.Errorf("evaluation capability %q references unknown evaluation %q", entry.ID, evaluationID)
		}
		if index > 0 && evaluations[index-1] == evaluationID {
			return CapabilityCoverage{}, fmt.Errorf("evaluation capability %q repeats evaluation %q", entry.ID, evaluationID)
		}
	}
	return CapabilityCoverage{ID: entry.ID, Area: entry.Area, Summary: strings.TrimSpace(entry.Summary), Tier: entry.Tier, Evaluations: evaluations}, nil
}

// coverageCatalogWire is the closed YAML shape for the promoted capability inventory.
type coverageCatalogWire struct {
	SchemaVersion int                      `yaml:"schema_version"`
	Capabilities  []capabilityCoverageWire `yaml:"capabilities"`
}

// capabilityCoverageWire is one serialized capability-to-evaluation mapping.
type capabilityCoverageWire struct {
	ID          string       `yaml:"id"`
	Area        string       `yaml:"area"`
	Summary     string       `yaml:"summary"`
	Tier        CoverageTier `yaml:"tier"`
	Evaluations []string     `yaml:"evaluations"`
}
