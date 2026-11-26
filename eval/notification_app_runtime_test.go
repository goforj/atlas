package eval

import (
	"go/parser"
	"go/token"
	"testing"
)

// TestNotificationAppRuntimeSourcesParse catches malformed supervisor programs before rendered calibration.
func TestNotificationAppRuntimeSourcesParse(t *testing.T) {
	contract := appResourceRuntimeContract("notification-app-runtime", "incidents", notificationAppRuntimeProbe, notificationAppRuntimeCommand)
	for name, source := range map[string]string{
		"notification_app_runtime_test.go": contract.supervisorFiles[0].body,
		"hello_world_cmd.go":               notificationAppRuntimeCommand,
	} {
		if _, err := parser.ParseFile(token.NewFileSet(), name, source, parser.AllErrors); err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
	}
}
