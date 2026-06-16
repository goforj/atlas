package guidelines

import (
	"fmt"
	"strings"

	"github.com/goforj/atlas/project"
)

// Compose builds the short generated guideline block for local agents.
func Compose(p project.Project) string {
	p = p.WithDiscoveredDefaults()

	var out strings.Builder
	out.WriteString("# GoForj Atlas\n\n")
	out.WriteString("Use Atlas and GoForj conventions when editing this project.\n\n")
	out.WriteString("## Project Shape\n\n")
	fmt.Fprintf(&out, "- project: `%s`\n", p.Name)
	fmt.Fprintf(&out, "- GoForj version: `%s`\n", fallback(p.GoForjVersion, "unknown"))
	out.WriteString("- default app composition lives in `app/`\n")
	out.WriteString("- default app binary entrypoint lives in `cmd/app/main.go`\n")
	out.WriteString("- named apps use `app/<name>/` and `cmd/<name>/main.go`\n")
	out.WriteString("- shared implementation and domain code live in `internal/`\n\n")

	out.WriteString("## Working Rules\n\n")
	out.WriteString("- Prefer `forj make:*` for framework scaffolding when a matching command exists.\n")
	out.WriteString("- Use `forj <app> make:*` when generated code belongs to a named app.\n")
	out.WriteString("- Keep business logic package-scoped under `internal/`; do not move domain code into `app/`.\n")
	out.WriteString("- Update app registration and Wire files through the selected app's composition points.\n")
	out.WriteString("- Keep MCP and Atlas operations read-only unless an explicit write feature is added later.\n")
	out.WriteString("- For GoForj framework validation renders, use `/tmp`, never the GoForj repo directory.\n\n")

	out.WriteString("## Apps\n\n")
	for _, app := range p.Apps {
		label := app.Name
		if app.Default {
			label += " (default)"
		}
		fmt.Fprintf(&out, "- `%s`: runtimes `%s`\n", label, strings.Join(app.Runtimes, "`, `"))
	}

	return strings.TrimSpace(out.String())
}

// fallback keeps generated guidance readable when GoForj supplies partial discovery data.
func fallback(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
