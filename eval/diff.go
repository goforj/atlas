package eval

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/pmezard/go-difflib/difflib"
)

const maxDiffSourceFileSize = 1 << 20
const maxDiffRetainedContentSize = 8 << 20

// projectDiffEntry retains enough trusted tree state to explain a candidate change without retaining large or binary file content.
type projectDiffEntry struct {
	mode     os.FileMode
	kind     string
	digest   string
	content  []byte
	link     string
	tooLarge bool
	binary   bool
}

// projectDiffSnapshot is the supervisor-owned baseline captured before an agent can mutate the candidate Project.
type projectDiffSnapshot map[string]projectDiffEntry

// buildProjectDiff compares the immutable baseline snapshot with the sealed candidate using a deterministic, bounded text projection.
func buildProjectDiff(baseline projectDiffSnapshot, finalRoot string) (string, error) {
	final, _, err := snapshotProjectForDiff(finalRoot)
	if err != nil {
		return "", fmt.Errorf("snapshot final Project: %w", err)
	}

	paths := make([]string, 0, len(baseline)+len(final))
	seen := map[string]bool{}
	for path := range baseline {
		seen[path] = true
		paths = append(paths, path)
	}
	for path := range final {
		if !seen[path] {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)

	var output strings.Builder
	for _, path := range paths {
		before, hadBefore := baseline[path]
		after, hasAfter := final[path]
		if hadBefore && hasAfter && equalProjectDiffEntry(before, after) {
			continue
		}
		if err := appendProjectDiff(&output, path, before, hadBefore, after, hasAfter); err != nil {
			return "", err
		}
		if output.Len() > maxArtifactFileSize {
			return "", fmt.Errorf("Project diff exceeds %d bytes", maxArtifactFileSize)
		}
	}
	return output.String(), nil
}

// projectChanges returns a compact semantic delta so verifiers never need to interpret display-oriented patch text.
func projectChanges(baseline projectDiffSnapshot, finalRoot string) ([]ProjectChange, error) {
	final, _, err := snapshotProjectForDiff(finalRoot)
	if err != nil {
		return nil, fmt.Errorf("snapshot final Project: %w", err)
	}
	paths := make([]string, 0, len(baseline)+len(final))
	seen := map[string]bool{}
	for path := range baseline {
		seen[path] = true
		paths = append(paths, path)
	}
	for path := range final {
		if !seen[path] {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	changes := make([]ProjectChange, 0, len(paths))
	for _, path := range paths {
		before, hadBefore := baseline[path]
		after, hasAfter := final[path]
		if hadBefore && hasAfter && equalProjectDiffEntry(before, after) {
			continue
		}
		change := ProjectChange{Path: path}
		if hadBefore {
			change.Before = projectPathState(before)
		}
		if hasAfter {
			change.After = projectPathState(after)
		}
		changes = append(changes, change)
	}
	return changes, nil
}

// projectPathState removes retained content while preserving the immutable identity relevant to ownership checks.
func projectPathState(entry projectDiffEntry) ProjectPathState {
	return ProjectPathState{Kind: entry.kind, Digest: entry.digest, Mode: uint32(entry.mode)}
}

// snapshotProjectForDiff walks without following links and returns the exact tree identity presented to the agent.
func snapshotProjectForDiff(root string) (projectDiffSnapshot, string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, "", err
	}
	var paths []string
	if err := filepath.WalkDir(root, func(path string, directoryEntry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path != root {
			paths = append(paths, path)
		}
		return nil
	}); err != nil {
		return nil, "", err
	}
	sort.Strings(paths)

	entries := projectDiffSnapshot{}
	retainedContentSize := int64(0)
	treeHash := sha256.New()
	for _, path := range paths {
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return nil, "", err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return nil, "", err
		}
		entry := projectDiffEntry{mode: info.Mode()}
		fmt.Fprintf(treeHash, "%s\x00%s\x00", filepath.ToSlash(relative), info.Mode().String())
		switch {
		case info.IsDir():
			entry.kind = "directory"
		case info.Mode()&os.ModeSymlink != 0:
			entry.kind = "symlink"
			entry.link, err = os.Readlink(path)
			if err != nil {
				return nil, "", err
			}
			entry.digest = digestDiffBytes([]byte(entry.link))
			fmt.Fprintf(treeHash, "%s\x00", entry.link)
		case info.Mode().IsRegular():
			entry.kind = "file"
			retainContent := info.Size() <= maxDiffSourceFileSize && retainedContentSize+info.Size() <= maxDiffRetainedContentSize
			entry.content, entry.digest, entry.tooLarge, entry.binary, err = readDiffFile(path, retainContent, treeHash)
			if err != nil {
				return nil, "", err
			}
			if retainContent && !entry.binary {
				retainedContentSize += int64(len(entry.content))
			}
		default:
			return nil, "", fmt.Errorf("unsupported Project entry %q with mode %s", filepath.ToSlash(relative), info.Mode())
		}
		entries[filepath.ToSlash(relative)] = entry
	}
	return entries, fmt.Sprintf("sha256:%x", treeHash.Sum(nil)), nil
}

// readDiffFile hashes all content while retaining only text selected by the snapshot's aggregate budget.
func readDiffFile(path string, retainContent bool, treeHash io.Writer) ([]byte, string, bool, bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, "", false, false, err
	}
	defer file.Close()
	hash := sha256.New()
	var content bytes.Buffer
	writers := []io.Writer{hash, treeHash}
	if retainContent {
		writers = append(writers, &content)
	}
	if _, err := io.Copy(io.MultiWriter(writers...), file); err != nil {
		return nil, "", false, false, err
	}
	body := content.Bytes()
	binary := retainContent && (bytes.IndexByte(body, 0) >= 0 || !utf8.Valid(body))
	if binary {
		body = nil
	}
	return body, fmt.Sprintf("sha256:%x", hash.Sum(nil)), !retainContent, binary, nil
}

// appendProjectDiff emits focused unified hunks while keeping tree collection and content policy supervisor-owned.
func appendProjectDiff(output *strings.Builder, path string, before projectDiffEntry, hadBefore bool, after projectDiffEntry, hasAfter bool) error {
	fmt.Fprintf(output, "diff --git %s %s\n", diffPath("a", path), diffPath("b", path))
	if hadBefore && (!hasAfter || before.mode.Perm() != after.mode.Perm()) {
		fmt.Fprintf(output, "old mode %06o\n", before.mode.Perm())
	}
	if hasAfter && (!hadBefore || before.mode.Perm() != after.mode.Perm()) {
		fmt.Fprintf(output, "new mode %06o\n", after.mode.Perm())
	}
	if hadBefore && hasAfter && before.kind != after.kind {
		fmt.Fprintf(output, "type changed from %s to %s\n", before.kind, after.kind)
	}
	if hadBefore && hasAfter && before.digest == after.digest && before.kind == after.kind {
		return nil
	}

	if (hadBefore && (before.binary || before.tooLarge)) || (hasAfter && (after.binary || after.tooLarge)) {
		fmt.Fprintf(output, "Binary or oversized files %s and %s differ\n", diffPath("a", path), diffPath("b", path))
		return nil
	}
	if (hadBefore && before.kind == "directory") || (hasAfter && after.kind == "directory") {
		return nil
	}
	oldContent, oldLabel := diffEntryContent(before, hadBefore, "a", path)
	newContent, newLabel := diffEntryContent(after, hasAfter, "b", path)
	patch, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
		A:        difflib.SplitLines(string(oldContent)),
		B:        difflib.SplitLines(string(newContent)),
		FromFile: oldLabel,
		ToFile:   newLabel,
		Context:  3,
	})
	if err != nil {
		return fmt.Errorf("render Project diff for %q: %w", path, err)
	}
	output.WriteString(patch)
	return nil
}

// diffEntryContent projects symlink targets as text while leaving directories to mode metadata.
func diffEntryContent(entry projectDiffEntry, exists bool, prefix, path string) ([]byte, string) {
	if !exists {
		return nil, "/dev/null"
	}
	if entry.kind == "symlink" {
		return []byte(entry.link + "\n"), diffPath(prefix, path)
	}
	return entry.content, diffPath(prefix, path)
}

// diffPath quotes paths so control characters cannot forge patch structure.
func diffPath(prefix, path string) string {
	return strconv.Quote(prefix + "/" + path)
}

// equalProjectDiffEntry compares both identity and display-relevant metadata.
func equalProjectDiffEntry(left, right projectDiffEntry) bool {
	return left.mode == right.mode && left.kind == right.kind && left.digest == right.digest
}

// digestDiffBytes returns the explicit digest form used throughout evaluation metadata.
func digestDiffBytes(body []byte) string {
	digest := sha256.Sum256(body)
	return fmt.Sprintf("sha256:%x", digest)
}
