// MacXVM - Slim Level Tests (MIT)
// https://github.com/shadow-x78/macxvm

package slim

import "testing"

func TestValidateLevel(t *testing.T) {
	if err := ValidateLevel(1); err != nil {
		t.Fatalf("ValidateLevel(1) = %v", err)
	}
	if err := ValidateLevel(2); err != nil {
		t.Fatalf("ValidateLevel(2) = %v", err)
	}
	for _, level := range []int{0, 3, -1} {
		if err := ValidateLevel(level); err == nil {
			t.Errorf("ValidateLevel(%d) = nil error, want an error", level)
		}
	}
}
