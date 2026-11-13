// Package exit defines exit codes and an error type that carries one.
package exit

import (
	"errors"
	"fmt"
)

// Class is both an error category and the process exit code.
type Class int

// Exit codes, aligned with the CLI contract.
const (
	OK         Class = 0
	User       Class = 1 // bad reference, missing tab, unknown profile
	Validation Class = 2 // bad flags or arguments
	Upstream   Class = 3 // the browser failed or is not reachable
	Permission Class = 4 // permission or timeout waiting for authorization

	// Semantic aliases
	NotFound Class = User
	Usage    Class = Validation
	Internal Class = Upstream
)

// Error carries a class and a human-actionable message.
type Error struct {
	Class Class
	Msg   string
}

func (e *Error) Error() string { return e.Msg }

// New builds a classified error.
func New(c Class, format string, a ...any) *Error {
	return &Error{Class: c, Msg: fmt.Sprintf(format, a...)}
}

// Code returns the exit code for any error (Upstream for unclassified ones).
func Code(err error) int {
	if err == nil {
		return int(OK)
	}
	var e *Error
	if errors.As(err, &e) {
		return int(e.Class)
	}
	return int(Upstream)
}

// CodeOf returns the exit code carried by err, or Upstream.
func CodeOf(err error) int {
	return Code(err)
}
