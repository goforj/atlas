package workflows

import (
	"strings"

	atlasdocs "github.com/goforj/atlas/docs"
	"github.com/goforj/atlas/project"
)

// VersionAlignmentResult compares project, Atlas, and docs versions.
type VersionAlignmentResult struct {
	ProjectGoForjVersion string             `json:"project_goforj_version,omitempty"`
	AtlasVersion         string             `json:"atlas_version,omitempty"`
	DocsVersion          string             `json:"docs_version,omitempty"`
	DocsRef              string             `json:"docs_ref,omitempty"`
	DocsRevision         string             `json:"docs_revision,omitempty"`
	DocsCommit           string             `json:"docs_commit,omitempty"`
	Aligned              bool               `json:"aligned"`
	Warnings             []VersionWarning   `json:"warnings,omitempty"`
	Manifest             atlasdocs.Manifest `json:"manifest"`
}

// VersionWarning describes one actionable version mismatch.
type VersionWarning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// VersionAlignment compares project version facts with docs metadata.
func VersionAlignment(p project.Project, atlasVersion string, manifest atlasdocs.Manifest) VersionAlignmentResult {
	result := VersionAlignmentResult{
		ProjectGoForjVersion: p.GoForjVersion,
		AtlasVersion:         atlasVersion,
		DocsVersion:          manifest.Version,
		DocsRef:              manifest.Ref,
		DocsRevision:         manifest.Revision,
		DocsCommit:           manifest.Commit,
		Manifest:             manifest,
		Aligned:              true,
	}
	projectVersion := strings.TrimSpace(p.GoForjVersion)
	docsVersion := firstNonEmptyString(manifest.GoForjVersion, manifest.Version)
	if projectVersion != "" && docsVersion != "" && projectVersion != docsVersion {
		result.warn("docs-version-mismatch", "Project targets GoForj "+projectVersion+" but the active docs bundle is "+docsVersion+".")
	}
	if projectVersion != "" && comparableAtlasVersion(atlasVersion) && projectVersion != atlasVersion {
		result.warn("cli-version-mismatch", "Project was rendered for GoForj "+projectVersion+" but the local Atlas/GoForj CLI reports "+atlasVersion+".")
	}
	if projectVersion != "" && strings.EqualFold(strings.TrimSpace(manifest.Ref), "main") && looksReleased(projectVersion) {
		result.warn("docs-main-for-release", "Docs are loaded from main while the project targets released GoForj "+projectVersion+".")
	}
	if strings.TrimSpace(manifest.Ref) == "" && strings.TrimSpace(manifest.Revision) == "" {
		result.warn("docs-ref-unknown", "The active docs bundle does not report a ref or revision.")
	}
	return result
}

func (r *VersionAlignmentResult) warn(code string, message string) {
	r.Aligned = false
	r.Warnings = append(r.Warnings, VersionWarning{Code: code, Message: message})
}

func comparableAtlasVersion(version string) bool {
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	if version == "" || version == "dev" || version == "test" {
		return false
	}
	first := version[0]
	return first >= '0' && first <= '9'
}

func looksReleased(version string) bool {
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	parts := strings.Split(version, ".")
	return len(parts) >= 2 && parts[0] != "" && parts[1] != ""
}
