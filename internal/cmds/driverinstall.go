// MacXVM - Driver Installation Helpers (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/shadow-x78/macxvm/internal/backup"
	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/sshx"
	"github.com/shadow-x78/macxvm/internal/stages"
	"github.com/shadow-x78/macxvm/internal/store"
	"github.com/shadow-x78/macxvm/resources"
)

func activeStageFile(cfg *config.VMConfig) string {
	if cfg.GPU.UsesGPU() {
		stage := cfg.GPU.DisplayStage
		if stage == "" {
			stage = config.StageDaily
		}
		return stages.ArgsPath(cfg.Dir(), stage)
	}
	return cfg.File("args.txt")
}

func uploadFile(cfg *config.VMConfig, local, remote string) error {
	if _, err := os.Stat(local); err != nil {
		return exitcode.New(exitcode.NotFound, "cannot upload %s: %v", local, err)
	}
	cmd := exec.Command("scp", sshx.SCPArgs(cfg, local, remote)...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return exitcode.New(exitcode.Generic, "cannot upload %s to the guest: %v", local, err)
	}
	return nil
}

func backupHostArtifacts(cfg *config.VMConfig, transaction string) (string, error) {
	dir := filepath.Join(cfg.Dir(), "driver", "install-backup", transaction)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", exitcode.New(exitcode.Generic, "cannot create the backup directory: %v", err)
	}
	sources := []struct {
		name string
		path string
	}{
		{"nvram.fd", cfg.File("nvram.fd")},
		{"driver/installed-manifest.json", cfg.File("driver/installed-manifest.json")},
	}
	for _, stage := range stages.Names {
		sources = append(sources, struct {
			name string
			path string
		}{stages.ArgsFileName(stage), stages.ArgsPath(cfg.Dir(), stage)})
	}
	opencoreDir := filepath.Join(cfg.Dir(), "opencore")
	if entries, err := os.ReadDir(opencoreDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			sources = append(sources, struct {
				name string
				path string
			}{"opencore/" + entry.Name(), filepath.Join(opencoreDir, entry.Name())})
		}
	}
	manifest := backup.Manifest{
		VMName:      cfg.Name,
		VMID:        cfg.ID,
		Transaction: transaction,
	}
	for _, source := range sources {
		info, err := os.Stat(source.path)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		sum, size, err := fileDigest(source.path)
		if err != nil {
			return "", exitcode.New(exitcode.Generic, "cannot hash %s: %v", source.path, err)
		}
		target := filepath.Join(dir, filepath.FromSlash(source.name))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return "", exitcode.New(exitcode.Generic, "cannot create the backup directory: %v", err)
		}
		raw, err := os.ReadFile(source.path)
		if err != nil {
			return "", exitcode.New(exitcode.Generic, "cannot read %s: %v", source.path, err)
		}
		if err := config.WriteFileAtomic(target, raw, 0o600); err != nil {
			return "", exitcode.New(exitcode.Generic, "cannot write %s: %v", target, err)
		}
		manifest.Entries = append(manifest.Entries, backup.Entry{Path: source.name, Size: size, SHA256: sum})
	}
	sort.Slice(manifest.Entries, func(i, j int) bool { return manifest.Entries[i].Path < manifest.Entries[j].Path })
	private, fingerprint, err := backup.EnsureKey()
	if err != nil {
		return "", exitcode.New(exitcode.Generic, "cannot use the backup key of the host: %v", err)
	}
	if _, err := backup.WriteManifest(dir, manifest, private); err != nil {
		return "", exitcode.New(exitcode.Generic, "cannot write the backup manifest: %v", err)
	}
	updated, err := store.Update(cfg.Name, func(current *config.VMConfig) error {
		current.BackupKeyFingerprint = fingerprint
		return nil
	})
	if err != nil {
		return "", err
	}
	*cfg = *updated
	return dir, nil
}

func fileDigest(path string) (string, int64, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", 0, err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), int64(len(raw)), nil
}

func guestInstallCommand(transaction, archive string) string {
	root := "/var/root/macxvm-mxgpu/" + transaction
	lines := []string{
		"set -e",
		"ROOT=" + root,
		"sudo /bin/mkdir -p \"$ROOT/staging\"",
		"sudo /usr/bin/install -m 0444 -o root -g wheel " + archive + " \"$ROOT/package.tar.gz\"",
		"sudo /bin/chmod 0755 \"$ROOT/staging\"",
		"sudo /usr/bin/tar -xzf \"$ROOT/package.tar.gz\" -C \"$ROOT/staging\"",
		"sudo /usr/bin/install -m 0444 -o root -g wheel /dev/stdin \"$ROOT/trust-root.txt\" <<'MACXVM_KEY'\n" +
			releasePublicKeyText() + "\nMACXVM_KEY",
		"sudo MACXVM_TRUST_ROOT=\"$ROOT/trust-root.txt\" /bin/sh \"$ROOT/staging/install.sh\" \"$ROOT/staging\"",
	}
	return strings.Join(lines, "\n")
}

func releasePublicKeyText() string {
	var out []string
	for _, line := range strings.Split(resources.MXGPUReleasePublicKey, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		out = append(out, trimmed)
	}
	return strings.Join(out, "\n")
}

func guestUninstallCommand(transaction string) string {
	root := "/var/root/macxvm-mxgpu/" + transaction
	return strings.Join([]string{
		"set -e",
		"ROOT=" + root,
		"if [ -x \"$ROOT/staging/uninstall.sh\" ]; then",
		"  sudo /bin/sh \"$ROOT/staging/uninstall.sh\" \"$ROOT/staging\"",
		"else",
		"  echo 'the guest holds no uninstall script of the driver' >&2",
		"  exit 1",
		"fi",
	}, "\n")
}
