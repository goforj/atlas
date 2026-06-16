package docs

import "context"

// Provider supplies versioned documentation to Atlas.
type Provider interface {
	// Manifest returns version and revision information for the docs set.
	Manifest(context.Context) (Manifest, error)
	// Documents returns the Markdown documents available to Atlas.
	Documents(context.Context) ([]Document, error)
}

// Manifest describes the docs set exposed by Atlas.
type Manifest struct {
	Version  string `json:"version"`
	Revision string `json:"revision"`
}

// Document is one Markdown document in a docs set.
type Document struct {
	Path    string            `json:"path"`
	Title   string            `json:"title"`
	Content string            `json:"content"`
	Tags    []string          `json:"tags,omitempty"`
	Meta    map[string]string `json:"meta,omitempty"`
}

// StaticProvider is an in-memory docs provider for embedded or test docs.
type StaticProvider struct {
	Docs     []Document
	DocsMeta Manifest
}

// Manifest returns the static docs manifest.
func (p StaticProvider) Manifest(context.Context) (Manifest, error) {
	return p.DocsMeta, nil
}

// Documents returns the static docs documents.
func (p StaticProvider) Documents(context.Context) ([]Document, error) {
	return append([]Document(nil), p.Docs...), nil
}
