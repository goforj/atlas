package docs

import (
	"context"
	"sort"
	"strings"
)

// SearchResult is one ranked docs search hit.
type SearchResult struct {
	Path    string `json:"path"`
	Title   string `json:"title"`
	Heading string `json:"heading"`
	Snippet string `json:"snippet"`
	Score   int    `json:"score"`
}

// SearchOptions controls docs search.
type SearchOptions struct {
	Query      string
	Limit      int
	TokenLimit int
}

// Search finds relevant docs sections.
func Search(ctx context.Context, provider Provider, opts SearchOptions) ([]SearchResult, error) {
	docs, err := provider.Documents(ctx)
	if err != nil {
		return nil, err
	}
	if opts.Limit <= 0 {
		opts.Limit = 5
	}
	if opts.TokenLimit <= 0 {
		opts.TokenLimit = 80
	}

	terms := queryTerms(opts.Query)
	results := []SearchResult{}
	for _, doc := range docs {
		for _, section := range ParseSections(doc) {
			score := scoreSection(section, terms)
			if score == 0 {
				continue
			}
			results = append(results, SearchResult{
				Path:    section.Path,
				Title:   section.Title,
				Heading: section.Heading,
				Snippet: limitWords(section.Body, opts.TokenLimit),
				Score:   score,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		if results[i].Path != results[j].Path {
			return results[i].Path < results[j].Path
		}
		return results[i].Heading < results[j].Heading
	})

	if len(results) > opts.Limit {
		results = results[:opts.Limit]
	}
	return results, nil
}

// ReadSection returns one section matching path and heading.
func ReadSection(ctx context.Context, provider Provider, path string, heading string, tokenLimit int) (Section, bool, error) {
	docs, err := provider.Documents(ctx)
	if err != nil {
		return Section{}, false, err
	}
	for _, doc := range docs {
		if doc.Path != path {
			continue
		}
		for _, section := range ParseSections(doc) {
			if strings.EqualFold(section.Heading, heading) || heading == "" {
				if tokenLimit > 0 {
					section.Body = limitWords(section.Body, tokenLimit)
				}
				return section, true, nil
			}
		}
	}
	return Section{}, false, nil
}

// ReadNeighborhood returns a section plus nearby heading sections.
func ReadNeighborhood(ctx context.Context, provider Provider, path string, heading string, before int, after int, tokenLimit int) ([]Section, error) {
	docs, err := provider.Documents(ctx)
	if err != nil {
		return nil, err
	}
	for _, doc := range docs {
		if doc.Path != path {
			continue
		}
		sections := ParseSections(doc)
		for i, section := range sections {
			if !strings.EqualFold(section.Heading, heading) {
				continue
			}
			start := max(0, i-before)
			end := min(len(sections), i+after+1)
			out := append([]Section(nil), sections[start:end]...)
			for i := range out {
				if tokenLimit > 0 {
					out[i].Body = limitWords(out[i].Body, tokenLimit)
				}
			}
			return out, nil
		}
	}
	return nil, nil
}

// ListHeadings returns the heading tree for path.
func ListHeadings(ctx context.Context, provider Provider, path string) ([]Section, error) {
	docs, err := provider.Documents(ctx)
	if err != nil {
		return nil, err
	}
	for _, doc := range docs {
		if doc.Path == path {
			return HeadingTree(doc), nil
		}
	}
	return nil, nil
}

// ExplainAPI maps common GoForj commands and paths to docs sections.
func ExplainAPI(ctx context.Context, provider Provider, query string) ([]SearchResult, error) {
	query = strings.TrimSpace(query)
	switch {
	case strings.Contains(query, "make:controller") || strings.Contains(query, "routes.go"):
		query += " app registration make commands routes"
	case strings.Contains(query, "make:job"):
		query += " app registration jobs wire make commands"
	case strings.Contains(query, "make:schedule") || strings.Contains(query, "schedules.go"):
		query += " app registration schedules make commands"
	case strings.Contains(query, "cmd/") || strings.Contains(query, "main.go"):
		query += " app architecture binary entrypoint"
	case strings.Contains(query, "migrations/"):
		query += " migrations app connection"
	}
	return Search(ctx, provider, SearchOptions{Query: query, Limit: 5, TokenLimit: 80})
}

// scoreSection weights titles and headings above body text to reduce token-heavy misses.
func scoreSection(section Section, terms []string) int {
	title := strings.ToLower(section.Title)
	heading := strings.ToLower(section.Heading)
	body := strings.ToLower(section.Body)
	path := strings.ToLower(section.Path)

	score := 0
	for _, term := range terms {
		if strings.Contains(title, term) {
			score += 8
		}
		if strings.Contains(heading, term) {
			score += 6
		}
		if strings.Contains(path, term) {
			score += 4
		}
		if strings.Contains(body, term) {
			score += 1
		}
	}
	return score
}

// queryTerms normalizes loose agent queries into stable search terms.
func queryTerms(query string) []string {
	fields := strings.Fields(strings.ToLower(query))
	seen := map[string]struct{}{}
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.Trim(field, "`'\".,:;()[]{}")
		if len(field) < 2 {
			continue
		}
		if _, ok := seen[field]; ok {
			continue
		}
		seen[field] = struct{}{}
		out = append(out, field)
	}
	return out
}

// limitWords enforces token discipline at the retrieval boundary.
func limitWords(value string, limit int) string {
	words := strings.Fields(value)
	if limit <= 0 || len(words) <= limit {
		return strings.TrimSpace(value)
	}
	return strings.Join(words[:limit], " ") + "..."
}
