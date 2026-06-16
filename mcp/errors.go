package mcp

import "errors"

// errNotFound gives read tools a consistent not-found message.
func errNotFound(name string) error {
	return errors.New(name + " not found")
}
