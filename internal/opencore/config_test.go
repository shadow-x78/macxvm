// MacXVM - OpenCore Config Tests (MIT)
// https://github.com/shadow-x78/macxvm

package opencore

import (
	"testing"

	"github.com/shadow-x78/macxvm/resources"
)

func TestEveryShippedConfigParses(t *testing.T) {
	configs := map[string]string{
		"mxgpu install":  resources.OpenCoreMXGPUInstall,
		"mxgpu daily":    resources.OpenCoreMXGPUDaily,
		"mxgpu recovery": resources.OpenCoreMXGPURecovery,
		"reims install":  resources.OpenCoreReimsInstall,
		"reims daily":    resources.OpenCoreReimsDaily,
		"reims recovery": resources.OpenCoreReimsRecovery,
	}
	for name, text := range configs {
		if _, err := ParseDict([]byte(text)); err != nil {
			t.Fatalf("the OpenCore config of %s does not parse: %v", name, err)
		}
	}
}
