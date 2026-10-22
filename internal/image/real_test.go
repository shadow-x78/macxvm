// MacXVM - Real Chunk List Test (MIT)
// https://github.com/shadow-x78/macxvm

package image

import (
	"os"
	"testing"
)

func TestTheRealChunkListOfApple(t *testing.T) {
	chunk := os.Getenv("MACXVM_TEST_CHUNKLIST")
	image := os.Getenv("MACXVM_TEST_IMAGE")
	if chunk == "" || image == "" {
		t.Skip("the paths of a real chunk list are not set")
	}
	if err := verifyChunkList(chunk, image); err != nil {
		t.Fatalf("verifyChunkList() = %v", err)
	}
	t.Log("the chunk list of Apple is valid")
}
