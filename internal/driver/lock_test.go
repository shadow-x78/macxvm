// MacXVM - Third Party Lock Tests (MIT)
// https://github.com/shadow-x78/macxvm

package driver

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func lockFixture(t *testing.T) *LockFile {
	t.Helper()
	digest := "0000000000000000000000000000000000000000000000000000000000000000"
	return &LockFile{
		Schema:      LockSchema,
		GeneratedAt: "2026-10-31",
		Policy: LockPolicy{
			Redistribution: "no binary is copied",
			Verification:   "the digest is written when the artifact is provisioned",
		},
		Components: []LockComponent{
			{
				ID:            "reference",
				Role:          "behavioural reference",
				Source:        "https://example.invalid/reference",
				Revision:      "v1",
				License:       "MIT",
				Redistributed: false,
				Provenance:    ProvenanceReference,
			},
			{
				ID:            "provisioned",
				Role:          "firmware",
				Source:        "https://example.invalid/firmware",
				Revision:      "v2",
				License:       "MIT",
				Redistributed: false,
				Provenance:    ProvenanceProvisioned,
				SHA256:        &digest,
			},
		},
	}
}

func TestValidateLockAcceptsACompleteFile(t *testing.T) {
	lock := lockFixture(t)
	if err := ValidateLock(lock); err != nil {
		t.Fatalf("the lock file is rejected: %v", err)
	}
	if len(lock.IDs()) != 2 {
		t.Fatalf("the lock file lists %v", lock.IDs())
	}
	if unprovisioned := lock.Unprovisioned(); len(unprovisioned) != 1 || unprovisioned[0] != "reference" {
		t.Fatalf("the unprovisioned components are %v", unprovisioned)
	}
}

func TestValidateLockRejectsTheBrokenFiles(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*LockFile)
	}{
		{"another schema", func(l *LockFile) { l.Schema = "other" }},
		{"no policy", func(l *LockFile) { l.Policy.Redistribution = "" }},
		{"no component", func(l *LockFile) { l.Components = nil }},
		{"a duplicate identity", func(l *LockFile) { l.Components[1].ID = l.Components[0].ID }},
		{"no source", func(l *LockFile) { l.Components[0].Source = "" }},
		{"an insecure source", func(l *LockFile) { l.Components[0].Source = "http://example.invalid/reference" }},
		{"no revision", func(l *LockFile) { l.Components[0].Revision = "" }},
		{"no license", func(l *LockFile) { l.Components[0].License = "" }},
		{"an unknown provenance", func(l *LockFile) { l.Components[0].Provenance = "unknown" }},
		{"a redistributed reference", func(l *LockFile) { l.Components[0].Redistributed = true }},
		{"a provisioned component without a digest", func(l *LockFile) { l.Components[1].SHA256 = nil }},
		{"a short digest", func(l *LockFile) {
			short := "abcd"
			l.Components[1].SHA256 = &short
		}},
		{"a digest that is not hexadecimal", func(l *LockFile) {
			bad := "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"
			l.Components[1].SHA256 = &bad
		}},
	}
	for _, item := range cases {
		lock := lockFixture(t)
		item.mutate(lock)
		if err := ValidateLock(lock); err == nil {
			t.Fatalf("the lock file with %s is accepted", item.name)
		}
	}
}

func TestParseLockRejectsBrokenJSON(t *testing.T) {
	if _, err := ParseLock([]byte("{")); err == nil {
		t.Fatal("the broken lock file is accepted")
	}
}

func TestTheLockFileOfTheRepositoryIsValid(t *testing.T) {
	path := filepath.Join("..", "..", "driver", "third_party.lock.json")
	lock, err := ReadLock(path)
	if err != nil {
		t.Fatalf("the lock file of the repository is rejected: %v", err)
	}
	if len(lock.Components) == 0 {
		t.Fatal("the lock file of the repository lists no component")
	}
	raw, err := json.Marshal(lock)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) == 0 {
		t.Fatal("the lock file of the repository is empty")
	}
}
