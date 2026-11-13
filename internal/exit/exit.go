// Package exit defines error classes that map to process exit codes, so every
// failure path is loud and structured rather than a bare panic or os.Exit(1).
package exit

import "fmt"

// Class is both an error category and the process exit code.
type Class int

const (
	OK         Class = iota // 0: success
	Usage                   // 1: bad invocation
	NotFound                // 2: a browser/profile/target was not present
	Upstream                // 3: the browser or OS returned an error we surface as-is
	Internal                // 4: a bug in bxw
	Permission              // 5: permission or timeout waiting for authorization
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

// Code returns the exit code for any error (Internal for unclassified ones).
func Code(err error) int {
	if err == nil {
		return int(OK)
	}
	if e, ok := err.(*Error); ok {
		return int(e.Class)
	}
	return int(Internal)
}
