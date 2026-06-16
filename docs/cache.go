package docs

import (
	"context"
	"errors"
	"sync"
)

// CachingProvider loads docs once and serves them from memory afterward.
type CachingProvider struct {
	Provider Provider

	mu        sync.Mutex
	loaded    bool
	manifest  Manifest
	documents []Document
}

// Manifest returns cached docs metadata.
func (p *CachingProvider) Manifest(ctx context.Context) (Manifest, error) {
	if err := p.load(ctx); err != nil {
		return Manifest{}, err
	}
	return p.manifest, nil
}

// Documents returns cached Markdown documents.
func (p *CachingProvider) Documents(ctx context.Context) ([]Document, error) {
	if err := p.load(ctx); err != nil {
		return nil, err
	}
	return append([]Document(nil), p.documents...), nil
}

// load keeps docs retrieval fast after the first successful provider read.
func (p *CachingProvider) load(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.loaded {
		return nil
	}

	if p.Provider == nil {
		return errors.New("docs provider is not configured")
	}
	documents, err := p.Provider.Documents(ctx)
	if err != nil {
		return err
	}
	manifest, err := p.Provider.Manifest(ctx)
	if err != nil {
		return err
	}
	p.documents = append([]Document(nil), documents...)
	p.manifest = manifest
	p.loaded = true
	return nil
}
