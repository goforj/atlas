package eval

import (
	"regexp"
	"strings"
	"unicode"
)

const redactedValue = "[REDACTED]"

var bearerSecretPattern = regexp.MustCompile(`(?i)\bbearer\s+[a-z0-9._~+/-]+=*`)
var assignmentSecretPattern = regexp.MustCompile(`(?i)\b(password|passwd|secret|token|authorization|cookie|api[_-]?key|client[_-]?secret)\s*[:=]\s*[^\s,;]+`)
var ansiControlPattern = regexp.MustCompile(`\x1b(?:\[[0-?]*[ -/]*[@-~]|\][^\x07]*(?:\x07|\x1b\\))`)

// Redactor removes known credentials and common secret-bearing values before persistence.
type Redactor struct {
	secrets []string
}

// NewRedactor snapshots non-empty secret values in longest-first order.
func NewRedactor(secrets []string) Redactor {
	redactor := Redactor{}
	for _, secret := range secrets {
		if secret != "" {
			redactor.secrets = append(redactor.secrets, secret)
		}
	}
	for left := 0; left < len(redactor.secrets); left++ {
		for right := left + 1; right < len(redactor.secrets); right++ {
			if len(redactor.secrets[right]) > len(redactor.secrets[left]) {
				redactor.secrets[left], redactor.secrets[right] = redactor.secrets[right], redactor.secrets[left]
			}
		}
	}
	return redactor
}

// Text redacts secrets and removes terminal or directional controls so human renderers receive inert content.
func (redactor Redactor) Text(value string) string {
	value = ansiControlPattern.ReplaceAllString(value, "")
	value = bearerSecretPattern.ReplaceAllString(value, "Bearer "+redactedValue)
	value = assignmentSecretPattern.ReplaceAllStringFunc(value, func(match string) string {
		separator := strings.IndexAny(match, ":=")
		if separator < 0 {
			return redactedValue
		}
		return match[:separator+1] + redactedValue
	})
	for _, secret := range redactor.secrets {
		value = strings.ReplaceAll(value, secret, redactedValue)
	}
	return strings.Map(func(character rune) rune {
		if character == '\n' || character == '\t' || !unicode.IsControl(character) && !isDirectionalControl(character) {
			return character
		}
		return -1
	}, value)
}

// containsSecret reports whether value still includes a registered credential.
func (redactor Redactor) containsSecret(value string) bool {
	for _, secret := range redactor.secrets {
		if strings.Contains(value, secret) {
			return true
		}
	}
	return false
}

// Event redacts a copy so callers cannot mutate or later reveal the persisted evidence.
func (redactor Redactor) Event(event Event) Event {
	redacted := event
	if event.Fields != nil {
		redacted.Fields = make(map[string]string, len(event.Fields))
		for key, value := range event.Fields {
			redacted.Fields[redactor.Text(key)] = redactor.Text(value)
		}
	}
	return redacted
}

// JSONValue recursively redacts string keys and values while preserving typed JSON structure.
func (redactor Redactor) JSONValue(value any) any {
	switch typed := value.(type) {
	case string:
		return redactor.Text(typed)
	case []any:
		redacted := make([]any, len(typed))
		for index, item := range typed {
			redacted[index] = redactor.JSONValue(item)
		}
		return redacted
	case map[string]any:
		redacted := make(map[string]any, len(typed))
		for key, item := range typed {
			redacted[redactor.Text(key)] = redactor.JSONValue(item)
		}
		return redacted
	default:
		return value
	}
}

// isDirectionalControl rejects bidi controls that can make inert evidence visually deceptive.
func isDirectionalControl(character rune) bool {
	return character >= '\u202a' && character <= '\u202e' || character >= '\u2066' && character <= '\u2069'
}
