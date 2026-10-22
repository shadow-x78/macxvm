// MacXVM - Exit Codes (MIT)
// https://github.com/shadow-x78/macxvm

package exitcode

import (
	"errors"
	"fmt"
)

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

type Error struct {
	Code int
	Msg  string
}

func (e *Error) Error() string { return e.Msg }

func New(code int, format string, a ...any) *Error {
	return &Error{Code: code, Msg: fmt.Sprintf(format, a...)}
}

func Exit(code int) *Error {
	return &Error{Code: code}
}

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
