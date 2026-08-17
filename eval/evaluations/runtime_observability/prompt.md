Repair the rendered App's disabled local Lighthouse inspect capture so a local operator can inspect runtime activity without starting Docker services.

Confirm that the generated `/metrics` surface is discoverable, `LIGHTHOUSE_INSPECT_ENABLED` enables local inspects for Lighthouse, and the operator documentation explains how HTTP, jobs, and scheduler work appear in inspect records. Use the App's normal build, test, and `route:list` workflow for evidence. Do not start long-running runtimes or add a service dependency merely to prove these configured surfaces.
