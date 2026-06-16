package docs

import (
	"strings"
)

// Section is one Markdown heading section.
type Section struct {
	Path    string `json:"path"`
	Title   string `json:"title"`
	Heading string `json:"heading"`
	Level   int    `json:"level"`
	Body    string `json:"body"`
}

// ParseSections parses a Markdown document into heading sections.
func ParseSections(doc Document) []Section {
	lines := strings.Split(doc.Content, "\n")
	sections := []Section{}
	current := Section{
		Path:    doc.Path,
		Title:   titleFor(doc),
		Heading: titleFor(doc),
		Level:   1,
	}
	var body []string

	flush := func() {
		current.Body = strings.TrimSpace(strings.Join(body, "\n"))
		if current.Heading != "" || current.Body != "" {
			sections = append(sections, current)
		}
		body = nil
	}

	seenHeading := false
	for _, line := range lines {
		level, heading, ok := headingLine(line)
		if ok {
			if seenHeading || len(body) > 0 {
				flush()
			}
			seenHeading = true
			current = Section{
				Path:    doc.Path,
				Title:   titleFor(doc),
				Heading: heading,
				Level:   level,
			}
			continue
		}
		body = append(body, line)
	}
	flush()

	return sections
}

// HeadingTree returns the headings for a document.
func HeadingTree(doc Document) []Section {
	all := ParseSections(doc)
	headings := make([]Section, 0, len(all))
	for _, section := range all {
		section.Body = ""
		headings = append(headings, section)
	}
	return headings
}

// headingLine accepts only standard ATX headings so section slicing stays predictable.
func headingLine(line string) (int, string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "#") {
		return 0, "", false
	}
	level := 0
	for level < len(trimmed) && trimmed[level] == '#' {
		level++
	}
	if level == 0 || level > 6 || level >= len(trimmed) || trimmed[level] != ' ' {
		return 0, "", false
	}
	return level, strings.TrimSpace(trimmed[level:]), true
}

// titleFor resolves a document title without making callers provide metadata.
func titleFor(doc Document) string {
	if strings.TrimSpace(doc.Title) != "" {
		return doc.Title
	}
	for _, section := range ParseSectionsWithoutTitle(doc) {
		if section.Level == 1 {
			return section.Heading
		}
	}
	return doc.Path
}

// ParseSectionsWithoutTitle exists to avoid recursive title discovery.
func ParseSectionsWithoutTitle(doc Document) []Section {
	lines := strings.Split(doc.Content, "\n")
	for _, line := range lines {
		level, heading, ok := headingLine(line)
		if ok {
			return []Section{{Path: doc.Path, Title: heading, Heading: heading, Level: level}}
		}
	}
	return nil
}
