package files

import (
	"strings"
)

// DefaultMarker is the marker used for generated Atlas guideline blocks.
const DefaultMarker = "goforj-atlas"

// Block returns a generated marker block for content.
func Block(marker string, content string) string {
	marker = normalizeMarker(marker)
	content = strings.TrimSpace(content)
	if content != "" {
		content += "\n"
	}
	return "<!-- " + marker + ":start -->\n" + content + "<!-- " + marker + ":end -->"
}

// MergeMarkerBlock replaces generated marker content while preserving user text.
func MergeMarkerBlock(existing string, marker string, generated string) string {
	marker = normalizeMarker(marker)
	block := Block(marker, generated)

	start := "<!-- " + marker + ":start -->"
	end := "<!-- " + marker + ":end -->"
	startIndex := strings.Index(existing, start)
	if startIndex < 0 {
		base := strings.TrimRight(existing, "\n")
		if base == "" {
			return block + "\n"
		}
		return base + "\n\n" + block + "\n"
	}

	endIndex := strings.Index(existing[startIndex:], end)
	if endIndex < 0 {
		base := strings.TrimRight(existing[:startIndex], "\n")
		if base == "" {
			return block + "\n"
		}
		return base + "\n\n" + block + "\n"
	}
	endIndex += startIndex + len(end)

	prefix := strings.TrimRight(existing[:startIndex], "\n")
	suffix := removeDuplicateBlocks(existing[endIndex:], marker)
	suffix = strings.TrimLeft(suffix, "\n")

	var out strings.Builder
	if prefix != "" {
		out.WriteString(prefix)
		out.WriteString("\n\n")
	}
	out.WriteString(block)
	if strings.TrimSpace(suffix) != "" {
		out.WriteString("\n\n")
		out.WriteString(strings.TrimRight(suffix, "\n"))
	}
	out.WriteString("\n")
	return out.String()
}

// RemoveMarkerBlock removes generated marker blocks while preserving surrounding user content.
func RemoveMarkerBlock(existing string, marker string) string {
	marker = normalizeMarker(marker)
	start := "<!-- " + marker + ":start -->"
	end := "<!-- " + marker + ":end -->"
	remaining := existing
	for {
		startIndex := strings.Index(remaining, start)
		if startIndex < 0 {
			break
		}
		endOffset := strings.Index(remaining[startIndex:], end)
		if endOffset < 0 {
			remaining = strings.TrimRight(remaining[:startIndex], "\n")
			break
		}
		endIndex := startIndex + endOffset + len(end)
		prefix := strings.TrimRight(remaining[:startIndex], "\n")
		suffix := strings.TrimLeft(remaining[endIndex:], "\n")
		switch {
		case prefix == "":
			remaining = suffix
		case suffix == "":
			remaining = prefix
		default:
			remaining = prefix + "\n\n" + suffix
		}
	}
	remaining = strings.TrimSpace(remaining)
	if remaining == "" {
		return ""
	}
	return remaining + "\n"
}

// removeDuplicateBlocks keeps repeated Atlas runs from accumulating generated blocks.
func removeDuplicateBlocks(content string, marker string) string {
	start := "<!-- " + marker + ":start -->"
	end := "<!-- " + marker + ":end -->"
	for {
		startIndex := strings.Index(content, start)
		if startIndex < 0 {
			return content
		}
		endIndex := strings.Index(content[startIndex:], end)
		if endIndex < 0 {
			return content[:startIndex]
		}
		endIndex += startIndex + len(end)
		content = content[:startIndex] + content[endIndex:]
	}
}

// normalizeMarker centralizes the default marker so callers can pass an empty marker safely.
func normalizeMarker(marker string) string {
	if strings.TrimSpace(marker) == "" {
		return DefaultMarker
	}
	return strings.TrimSpace(marker)
}
