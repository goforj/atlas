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
	out.WriteString("Use local Project evidence and GoForj's supported workflows before inventing framework conventions.\n\n")
	out.WriteString("## Project Shape\n\n")
	fmt.Fprintf(&out, "- project: `%s`\n", p.Name)
	fmt.Fprintf(&out, "- GoForj version: `%s`\n", fallback(p.GoForjVersion, "unknown"))
	out.WriteString("- default app composition lives in `app/`\n")
	out.WriteString("- default app binary entrypoint lives in `cmd/app/main.go`\n")
	out.WriteString("- named apps use `app/<name>/` and `cmd/<name>/main.go`\n")
	out.WriteString("- shared implementation and domain code live in `internal/`\n\n")

	out.WriteString("## Framework Workflow\n\n")
	out.WriteString("- Inspect `.goforj.yml`, the owning App, nearby packages, and `forj` command help before changing framework structure.\n")
	out.WriteString("- Before hand-writing a controller, command, job, schedule, event, subscriber, model, migration, or named queue, use the matching `forj make:*` generator when the artifact should participate in App registration.\n")
	out.WriteString("- The default App uses `forj make:<artifact> <name>`. An additional App uses `forj <app> make:<artifact> <name>`, for example `forj admin make:controller reports`.\n")
	out.WriteString("- After generation, inspect the generated file and every registration or Wire file changed by the command before adding behavior. Never edit `wire_gen.go`; regenerate Wire through GoForj.\n")
	out.WriteString("- If an artifact intentionally should not be registered, inspect generator help and local conventions first, then use a manual implementation only when that ownership choice is clear.\n\n")

	out.WriteString("## Ownership\n\n")
	out.WriteString("- Keep `app/` focused on App composition and transport registration. Put shared implementation and domain behavior in focused packages under `internal/`.\n")
	out.WriteString("- Start with one cohesive package for a responsibility and add files within it. Create a subpackage only when it has a cohesive API and can stand on its own; do not create category packages such as `services`, `handlers`, `models`, `types`, or `utils` merely to sort code.\n")
	out.WriteString("- Keep controllers, commands, jobs, and schedules thin; invoke services for application behavior.\n")
	out.WriteString("- Keep database access behind repositories or equivalent domain ports, and propagate cancellation through connection-backed work.\n")
	out.WriteString("- Treat generated registration points as in-use framework code: preserve the generated integration unless the requested design deliberately replaces it.\n\n")

	out.WriteString("## Evidence And Validation\n\n")
	out.WriteString("- Prefer local configuration, generated source, CLI inspection commands, and focused tests as evidence for how this Project works.\n")
	out.WriteString("- Use Atlas project inspection and version-aware documentation when available. Otherwise consult the documentation matching the Project's GoForj version at `https://goforj.dev`; ask before inventing a convention when local and documented evidence are insufficient.\n")
	out.WriteString("- Run focused tests for changed packages, then the Project's relevant GoForj build or broader test command.\n")
	out.WriteString("- Keep MCP and Atlas operations read-only unless an explicit write feature is added later.\n")
	out.WriteString("- For GoForj framework validation renders, use `/tmp`, never the GoForj repo directory.\n\n")
	out.WriteString("## Capturing Project Knowledge\n\n")
	out.WriteString("- When the user teaches a durable repo-specific convention, workflow, command, or review expectation, briefly ask whether it should become a project-owned Atlas skill in `.ai/skills/<name>/SKILL.md`.\n")
	out.WriteString("- Suggest this only for patterns likely to matter again; do not suggest skills for one-off preferences or temporary debugging steps.\n")
	out.WriteString("- Keep project-owned skills short, specific, and focused on what agents should do differently in this codebase.\n\n")

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
