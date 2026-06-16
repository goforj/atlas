package docs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// EnvPath is the development override for live GoForj docs.
const EnvPath = "GOFORJ_DOCS_PATH"

// FSProvider loads Markdown docs from a filesystem directory.
type FSProvider struct {
	Root     string
	Version  string
	Revision string
}

// ProviderFromEnv returns an FSProvider when GOFORJ_DOCS_PATH is set.
func ProviderFromEnv(version string) (Provider, bool) {
	root := strings.TrimSpace(os.Getenv(EnvPath))
	if root == "" {
		return nil, false
	}
	return FSProvider{Root: root, Version: version}, true
}

// Manifest returns a checksum-backed docs manifest.
func (p FSProvider) Manifest(ctx context.Context) (Manifest, error) {
	docs, err := p.Documents(ctx)
	if err != nil {
		return Manifest{}, err
	}
	hash := sha256.New()
	for _, doc := range docs {
		hash.Write([]byte(doc.Path))
		hash.Write([]byte{0})
		hash.Write([]byte(doc.Content))
		hash.Write([]byte{0})
	}
	revision := p.Revision
	if revision == "" {
		revision = hex.EncodeToString(hash.Sum(nil))[:12]
	}
	return Manifest{Version: p.Version, Revision: revision}, nil
}

// Documents loads Markdown documents from the provider root.
func (p FSProvider) Documents(ctx context.Context) ([]Document, error) {
	root, err := filepath.Abs(docsRoot(p.Root))
	if err != nil {
		return nil, err
	}

	docs := []Document{}
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if entry.IsDir() {
			name := entry.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "dist" {
				if path != root {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), ".md") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		docs = append(docs, Document{
			Path:    rel,
			Title:   firstHeading(string(content), rel),
			Content: string(content),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(docs, func(i, j int) bool {
		return docs[i].Path < docs[j].Path
	})
	return docs, nil
}

// firstHeading gives filesystem docs stable titles without requiring frontmatter.
func firstHeading(content string, fallback string) string {
	for _, line := range strings.Split(content, "\n") {
		level, heading, ok := headingLine(line)
		if ok && level == 1 {
			return heading
		}
	}
	return fallback
}

func docsRoot(root string) string {
	if dirExists(filepath.Join(root, "docs")) {
		return filepath.Join(root, "docs")
	}
	return root
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
