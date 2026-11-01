// MacXVM - Driver Package Helpers (MIT)
// https://github.com/shadow-x78/macxvm

package driver

import (
	"bytes"
	"encoding/json"
	"fmt"
)

func jsonUnmarshalStrict(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	return nil
}

func Info(manifest *Manifest) string {
	return fmt.Sprintf("MXGPU package format %d, key %s, target %s/%s, %d files",
		manifest.FormatVersion, manifest.KeyID, manifest.Target.OS, manifest.Target.Arch, len(manifest.Files))
}
