// MacXVM - Guest Installation Contract Tests (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"strings"
	"testing"
)

func TestGuestInstallKeepsTheTrustRoot(t *testing.T) {
	script := guestInstallCommand("tx", "/var/tmp/pkg.tar.gz")
	if !strings.Contains(script, "MACXVM_TRUST_ROOT=") {
		t.Fatal("the guest command does not hand the trust root to the installer")
	}
	if !strings.Contains(script, "trust-root.txt") {
		t.Fatal("the guest command does not write the trust root")
	}
	if !strings.Contains(script, releasePublicKeyText()) {
		t.Fatal("the guest command does not carry the key of the release")
	}
}

func TestGuestInstallRunsTheScriptsOfThePackage(t *testing.T) {
	script := guestInstallCommand("tx", "/var/tmp/pkg.tar.gz")
	if !strings.Contains(script, "/bin/sh \"$ROOT/staging/install.sh\"") {
		t.Fatal("the guest command does not run the install script of the package")
	}
	if strings.Contains(script, "/bin/sh \"$ROOT/staging/verify.sh\"") {
		t.Fatal("the guest command still runs a verification step of its own")
	}
}

func TestTheReleaseKeyIsHexWithoutTheComments(t *testing.T) {
	text := releasePublicKeyText()
	if strings.Contains(text, "#") {
		t.Fatalf("the release key still holds a comment: %q", text)
	}
	if strings.Contains(text, " ") || strings.Contains(text, "\n") {
		t.Fatalf("the release key is not one line of hex: %q", text)
	}
	if len(text) != 64 {
		t.Fatalf("the release key is %d hex characters, want 64", len(text))
	}
}
