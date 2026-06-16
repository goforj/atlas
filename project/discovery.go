package project

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// moduleLinePattern extracts the module path without needing a full Go module parser.
var moduleLinePattern = regexp.MustCompile(`^module\s+(.+)$`)

// Discover inspects a GoForj project root and returns the facts Atlas needs.
func Discover(root string) (Project, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Project{}, err
	}

	project := Project{
		Root:       absRoot,
		Name:       filepath.Base(absRoot),
		GoVersion:  discoverGoVersion(absRoot),
		Components: discoverComponents(absRoot),
		Apps:       discoverApps(absRoot),
	}
	if module := discoverModule(absRoot); module != "" {
		project.Name = moduleName(module)
	}

	return project.WithDiscoveredDefaults(), nil
}

// discoverModule reads go.mod directly so Atlas can stay lightweight.
func discoverModule(root string) string {
	file, err := os.Open(filepath.Join(root, "go.mod"))
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		matches := moduleLinePattern.FindStringSubmatch(scanner.Text())
		if len(matches) == 2 {
			return strings.TrimSpace(matches[1])
		}
	}
	return ""
}

// discoverGoVersion reads the module's Go version without shelling out to `go`.
func discoverGoVersion(root string) string {
	file, err := os.Open(filepath.Join(root, "go.mod"))
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "go ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "go "))
		}
	}
	return ""
}

// discoverComponents reads the current simple component shapes from `.goforj.yml`.
func discoverComponents(root string) []string {
	configPath := filepath.Join(root, ".goforj.yml")
	content, err := os.ReadFile(configPath)
	if err != nil {
		return nil
	}

	var components []string
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	inComponents := false
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "components:") {
			inComponents = true
			inline := strings.TrimSpace(strings.TrimPrefix(trimmed, "components:"))
			components = append(components, parseInlineList(inline)...)
			continue
		}
		if inComponents {
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			if !strings.HasPrefix(trimmed, "-") && !strings.HasPrefix(line, " ") {
				break
			}
			if strings.HasPrefix(trimmed, "-") {
				value := strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
				if value != "" {
					components = append(components, strings.Trim(value, `"'`))
				}
			}
		}
	}

	return uniqueStrings(components)
}

// discoverApps follows GoForj conventions instead of requiring app config boilerplate.
func discoverApps(root string) []App {
	apps := []App{}
	if fileExists(filepath.Join(root, "cmd", DefaultAppName, "main.go")) || dirExists(filepath.Join(root, "app")) {
		apps = append(apps, App{Name: DefaultAppName, Default: true})
	}

	cmdEntries, err := os.ReadDir(filepath.Join(root, "cmd"))
	if err == nil {
		for _, entry := range cmdEntries {
			if !entry.IsDir() || entry.Name() == DefaultAppName {
				continue
			}
			name := entry.Name()
			if fileExists(filepath.Join(root, "cmd", name, "main.go")) && dirExists(filepath.Join(root, "app", name)) {
				apps = append(apps, App{Name: name})
			}
		}
	}

	if len(apps) == 0 && fileExists(filepath.Join(root, "main.go")) {
		apps = append(apps, App{Name: DefaultAppName, Default: true})
	}

	sort.Slice(apps, func(i, j int) bool {
		if apps[i].Default != apps[j].Default {
			return apps[i].Default
		}
		return apps[i].Name < apps[j].Name
	})

	return apps
}

// parseInlineList supports compact YAML lists used in early project configs.
func parseInlineList(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	value = strings.Trim(value, "[]")
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(strings.Trim(part, `"'`))
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// uniqueStrings preserves discovery order while removing duplicate components.
func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

// moduleName derives a friendly project name from a module path.
func moduleName(module string) string {
	module = strings.TrimSuffix(module, "/")
	if module == "" {
		return ""
	}
	parts := strings.Split(module, "/")
	return parts[len(parts)-1]
}

// fileExists keeps project discovery tolerant of partially rendered projects.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// dirExists keeps project discovery tolerant of partially rendered projects.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
