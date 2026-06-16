package docs

import (
	"context"
	"testing"
)

func TestSearchRanksHeadingMatches(t *testing.T) {
	provider := fixtureProvider()
	results, err := Search(context.Background(), provider, SearchOptions{
		Query: "make controller marketplace app",
		Limit: 3,
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results")
	}
	if results[0].Heading != "Make Commands" {
		t.Fatalf("expected Make Commands first, got %#v", results[0])
	}
}

func TestReadSection(t *testing.T) {
	section, ok, err := ReadSection(context.Background(), fixtureProvider(), "apps.md", "App Architecture", 6)
	if err != nil {
		t.Fatalf("read section failed: %v", err)
	}
	if !ok {
		t.Fatal("expected section")
	}
	if section.Heading != "App Architecture" {
		t.Fatalf("unexpected heading %q", section.Heading)
	}
}

func TestReadNeighborhood(t *testing.T) {
	sections, err := ReadNeighborhood(context.Background(), fixtureProvider(), "apps.md", "App Registration", 1, 1, 20)
	if err != nil {
		t.Fatalf("read neighborhood failed: %v", err)
	}
	if len(sections) != 3 {
		t.Fatalf("expected 3 sections, got %d", len(sections))
	}
}

func TestListHeadings(t *testing.T) {
	headings, err := ListHeadings(context.Background(), fixtureProvider(), "apps.md")
	if err != nil {
		t.Fatalf("list headings failed: %v", err)
	}
	if len(headings) != 4 {
		t.Fatalf("expected 4 headings, got %d", len(headings))
	}
}

func fixtureProvider() StaticProvider {
	return StaticProvider{
		DocsMeta: Manifest{Version: "0.18.0", Revision: "test"},
		Docs: []Document{
			{
				Path:  "apps.md",
				Title: "Apps",
				Content: `# Apps

## App Architecture

Use cmd/app/main.go, app/, app/<name>/, and internal/.

## App Registration

Routes and commands register through app routes and app wire files.

## Make Commands

Use forj marketplace make:controller checkout for named app controller scaffolding.
`,
			},
		},
	}
}
