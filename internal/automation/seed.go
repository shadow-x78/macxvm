// MacXVM - Installer Seed ISO (MIT)
// https://github.com/shadow-x78/macxvm

package automation

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const SeedVolume = "macxvm"
const SeedScript = "/volumes/" + SeedVolume + "/setup.sh"

func seedScript(diskGiB int) string {
	expected := int64(diskGiB) * 1024 * 1024 * 1024
	low := expected * 85 / 100
	high := expected * 115 / 100
	return fmt.Sprintf(`#!/bin/bash
# MacXVM installer seed: erase exactly one internal disk matching the VM size.
EXPECTED=%d
LOW=%d
HIGH=%d
FOUND=""
COUNT=0
for D in $(diskutil list internal | awk '/^\/dev\/disk/{print $1}'); do
  SIZE=$(diskutil info -plist "$D" | plutil -extract TotalSize raw -o - - 2>/dev/null)
  [ -n "$SIZE" ] || continue
  if [ "$SIZE" -ge "$LOW" ] && [ "$SIZE" -le "$HIGH" ]; then
    FOUND="$D"
    COUNT=$((COUNT + 1))
  fi
done
if [ "$COUNT" -eq 1 ]; then
  if diskutil eraseDisk APFS MacHDD GPT "$FOUND"; then
    echo RESULT 777
  else
    echo RESULT 111
  fi
elif [ "$COUNT" -eq 0 ]; then
  echo RESULT 222
else
  echo RESULT 333
fi
`, expected, low, high)
}

// WriteSeed builds a small ISO with setup.sh. The guest mounts it read-only,
// so no arbitrary host file is shared and the script is fixed by the tool.
func WriteSeed(path string, diskGiB int) error {
	if diskGiB <= 0 {
		return fmt.Errorf("invalid disk size")
	}
	tool, err := exec.LookPath("genisoimage")
	if err != nil {
		tool, err = exec.LookPath("mkisofs")
	}
	if err != nil {
		return fmt.Errorf("installer automation needs genisoimage or mkisofs: %w", err)
	}
	dir, err := os.MkdirTemp("", "macxvm-seed-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	script := filepath.Join(dir, "setup.sh")
	if err := os.WriteFile(script, []byte(seedScript(diskGiB)), 0o755); err != nil {
		return err
	}
	if err := os.Chmod(script, 0o755); err != nil {
		return err
	}
	partial := path + ".partial"
	_ = os.Remove(partial)
	cmd := exec.Command(tool, "-quiet", "-R", "-V", SeedVolume, "-o", partial, dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.Remove(partial)
		return fmt.Errorf("cannot build the installer seed ISO: %v (%s)", err, strings.TrimSpace(string(out)))
	}
	if err := os.Rename(partial, path); err != nil {
		return err
	}
	return nil
}
