package eval

import "strconv"

// appResourceRuntimeContract verifies a resource through the generated App command and its real Wire graph.
func appResourceRuntimeContract(id, packageName, testSource, commandSource string) commandContract {
	return commandContract{
		id:              id,
		arguments:       []string{"go", "test", "./internal/" + packageName, "-run", "^TestAtlasResourceApp$", "-count=1"},
		supervisorFiles: []supervisorFile{{path: "internal/" + packageName + "/atlas_eval_app_runtime_test.go", body: testSource + appResourceRuntimeHelpers + "\nconst atlasAppCommandSource = " + strconv.Quote(commandSource) + "\n"}},
	}
}

// appResourceRuntimeHelpers keeps supervisor mutations limited to an existing generated command in the isolated clone.
const appResourceRuntimeHelpers = `
// atlasAppRoot locates the generated Project from the supervised resource package.
func atlasAppRoot(t *testing.T) string {
    t.Helper()
    root, err := filepath.Abs(filepath.Join("..", ".."))
    if err != nil { t.Fatal(err) }
    if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil { t.Fatalf("generated Project root: %v", err) }
    return root
}

// atlasBuildResourceApp makes the real App consume the resource through its existing command registration.
func atlasBuildResourceApp(t *testing.T, root string, env []string) {
    t.Helper()
    path := filepath.Join(root, "internal", "cmd", "hello_world_cmd.go")
    if _, err := os.Stat(path); err != nil { t.Fatalf("existing App command: %v", err) }
    if err := os.WriteFile(path, []byte(atlasAppCommandSource), 0600); err != nil { t.Fatal(err) }
    if output, err := atlasRunResourceTool(t, root, env, "forj", "build"); err != nil { t.Fatalf("build resource App: %v\n%s", err, output) }
}

// atlasRunResourceApp observes the generated App after Wire has resolved the agent's registered providers.
func atlasRunResourceApp(t *testing.T, root string, env []string) (string, error) {
    t.Helper()
    output, err := atlasRunResourceTool(t, root, env, "go", "run", "./cmd/app", "hello:world")
    return strings.TrimSpace(output), err
}

// atlasResourceOutput bounds shared stdout and stderr before retaining subprocess output.
type atlasResourceOutput struct {
    body strings.Builder
    exceeded bool
    cancel context.CancelFunc
}

// Write cancels a noisy subprocess as soon as its combined output exceeds the verifier ceiling.
func (output *atlasResourceOutput) Write(body []byte) (int, error) {
    size := len(body)
    remaining := (1 << 20) - output.body.Len()
    if size > remaining {
        output.body.Write(body[:remaining])
        output.exceeded = true
        output.cancel()
    } else {
        output.body.Write(body)
    }
    return size, nil
}

// atlasRunResourceTool preserves supervisor-selected executables and bounds every nested resource process.
func atlasRunResourceTool(t *testing.T, root string, env []string, tool string, args ...string) (string, error) {
    t.Helper()
    executable := os.Getenv("ATLAS_VERIFIER_" + strings.ToUpper(tool))
    if executable == "" { executable = tool }
    ctx, cancel := context.WithCancel(t.Context())
    defer cancel()
    command := exec.CommandContext(ctx, executable, args...)
    command.Dir = root
    command.Env = append(os.Environ(), env...)
    command.WaitDelay = 2 * time.Second
    output := &atlasResourceOutput{cancel: cancel}
    command.Stdout = output
    command.Stderr = output
    err := command.Run()
    if output.exceeded { t.Fatalf("resource tool %s output exceeds %d bytes", tool, 1 << 20) }
    return output.body.String(), err
}
`
