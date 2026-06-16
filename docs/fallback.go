package docs

import (
	"context"
	"errors"
)

// DefaultProvider returns Atlas docs using a local override or cached git docs.
func DefaultProvider(version string) Provider {
	providers := []Provider{}
	if provider, ok := ProviderFromEnv(version); ok {
		providers = append(providers, provider)
	}
	providers = append(providers, NewGitProvider(version))
	return &CachingProvider{Provider: FallbackProvider{Providers: providers}}
}

// FallbackProvider tries docs providers in order until one returns documents.
type FallbackProvider struct {
	Providers []Provider
}

// Manifest returns metadata for the first provider that can return documents.
func (p FallbackProvider) Manifest(ctx context.Context) (Manifest, error) {
	provider, err := p.provider(ctx)
	if err != nil {
		return Manifest{}, err
	}
	return provider.Manifest(ctx)
}

// Documents returns documents from the first available provider.
func (p FallbackProvider) Documents(ctx context.Context) ([]Document, error) {
	provider, err := p.provider(ctx)
	if err != nil {
		return nil, err
	}
	return provider.Documents(ctx)
}

// provider picks the first source with real documents so broken overrides do not hide git docs.
func (p FallbackProvider) provider(ctx context.Context) (Provider, error) {
	var lastErr error
	for _, provider := range p.Providers {
		if provider == nil {
			continue
		}
		documents, err := provider.Documents(ctx)
		if err != nil {
			lastErr = err
			continue
		}
		if len(documents) > 0 {
			return provider, nil
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errors.New("no docs providers available")
}
