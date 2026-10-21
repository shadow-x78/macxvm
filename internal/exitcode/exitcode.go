// MacXVM - Exit Codes (MIT)
// https://github.com/shadow-x78/macxvm

// Package exitcode provides the error type that carries a CLI exit code.
package exitcode

import (
	"errors"
	"fmt"
)

// Exit codes used by the macxvm CLI.
const (
	OK              = 0
	Generic         = 1
	Usage           = 2
	NotFound        = 3
	AlreadyRunning  = 4
	NotRunning      = 5
	UnsupportedHost = 6
	Timeout         = 7
	BadArgsFile     = 8
	ProfileMismatch = 9
)

// Error is an error with an associated process exit code.
type Error struct {
	Code int
	Msg  string
}

// Error returns the message without the exit code.
func (e *Error) Error() string { return e.Msg }

// New returns an *Error with the given code and a formatted message.
func New(code int, format string, a ...any) *Error {
	return &Error{Code: code, Msg: fmt.Sprintf(format, a...)}
}

// Exit returns an error that carries only an exit code. The command line
// prints no message for it, so the exit code of a remote command can be
// passed through unchanged.
func Exit(code int) *Error {
	return &Error{Code: code}
}

// Code returns the exit code carried by err. Any error that is not an
// *Error maps to the generic failure code.
func Code(err error) int {
	if err == nil {
		return OK
	}
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return Generic
}
