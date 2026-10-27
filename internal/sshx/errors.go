// MacXVM - SSH Failure Classification (MIT)
// https://github.com/shadow-x78/macxvm

package sshx

import (
	"fmt"
	"strings"
)

type Failure string

const (
	PortUnavailable Failure = "port-unavailable"
	BootTimeout     Failure = "boot-timeout"
	Authentication  Failure = "authentication"
	HostKeyMismatch Failure = "host-key-mismatch"
	SudoMissing     Failure = "sudo-missing"
	Unknown         Failure = "unknown"
)

type Error struct {
	Kind Failure
	VM   string
	Err  error
}

func (e *Error) Error() string {
	action := "inspect the SSH error and the guest console"
	switch e.Kind {
	case PortUnavailable:
		action = "enable Remote Login in the guest and verify its loopback SSH forwarding"
	case BootTimeout:
		action = "inspect the guest with 'macxvm open'; SSH did not become ready before the deadline"
	case Authentication:
		action = "resume 'macxvm base bootstrap' to install the MacXVM key; do not disable host-key checks"
	case HostKeyMismatch:
		action = "verify the guest identity; if the change is expected, remove only this VM's known_hosts and trust it again"
	case SudoMissing:
		action = "resume 'macxvm base bootstrap' to repair the validated sudoers entry"
	}
	if e.Err != nil {
		return fmt.Sprintf("SSH %s for '%s': %v; %s", e.Kind, e.VM, e.Err, action)
	}
	return fmt.Sprintf("SSH %s for '%s'; %s", e.Kind, e.VM, action)
}

func (e *Error) Unwrap() error { return e.Err }

func Classify(text string) Failure {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "remote host identification has changed"),
		strings.Contains(lower, "host key verification failed"),
		strings.Contains(lower, "offending key"),
		strings.Contains(lower, "host key mismatch"):
		return HostKeyMismatch
	case strings.Contains(lower, "permission denied"),
		strings.Contains(lower, "authentication failed"),
		strings.Contains(lower, "too many authentication failures"):
		return Authentication
	case strings.Contains(lower, "a password is required"),
		strings.Contains(lower, "a terminal is required to read the password"),
		strings.Contains(lower, "no password was provided"),
		strings.Contains(lower, "not in the sudoers"),
		strings.Contains(lower, "sudo -n"):
		return SudoMissing
	case strings.Contains(lower, "connection refused"),
		strings.Contains(lower, "no route to host"),
		strings.Contains(lower, "connection timed out"),
		strings.Contains(lower, "operation timed out"),
		strings.Contains(lower, "kex_exchange_identification"):
		return PortUnavailable
	default:
		return Unknown
	}
}
