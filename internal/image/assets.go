// MacXVM - Asset Fetch (MIT)
// https://github.com/shadow-x78/macxvm

package image

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/hostcheck"
	"github.com/shadow-x78/macxvm/resources"
)

func bootAssetURL(key string) (string, error) {
	for _, line := range strings.Split(resources.BootAssets, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		if len(parts) == 2 && parts[0] == key {
			return strings.TrimSpace(parts[1]), nil
		}
	}
	return "", exitcode.New(exitcode.Generic, "the pinned source of '%s' is missing", key)
}

const (
	FirmwareName = "OVMF_CODE_4M.fd"
	VarsName     = "OVMF_VARS-1920x1080.fd"
)

func fetchInstaller(opts Options) ([]AssetRecord, string, error) {
	rel, err := lookupRelease(opts.OS)
	if err != nil {
		return nil, "", err
	}
	img := fileIn(opts.Dir, InstallerName)
	dmg := fileIn(opts.Dir, "BaseSystem.dmg")
	chunklist := fileIn(opts.Dir, "BaseSystem.chunklist")

	client := newClient()
	product := "macOS"
	fetch := func() error {
		session, err := newSession(client)
		if err != nil {
			return err
		}
		info, err := fetchInfo(client, session, rel)
		if err != nil {
			return err
		}
		if info.Product != "" {
			product = info.Product
		}
		describe(fmt.Sprintf("the recovery image of %s from Apple: %s", info.Product, info.ImageURL))
		if err := downloadAsset(client, info.ChunkURL, info.ChunkToken, chunklist); err != nil {
			return err
		}
		return downloadAsset(client, info.ImageURL, info.ImageToken, dmg)
	}
	if !exists(dmg) || !exists(chunklist) {
		if err := fetch(); err != nil {
			return nil, "", err
		}
	} else {
		describe("checking the locally cached Apple recovery image")
	}
	if err := verifyChunkList(chunklist, dmg); err != nil {
		describe("the cached recovery image failed verification; fetching a fresh copy")
		if err := fetch(); err != nil {
			return nil, "", err
		}
		if err := verifyChunkList(chunklist, dmg); err != nil {
			return nil, "", err
		}
	}
	describe("the signature and every chunk of the Apple image are verified")
	if err := convertDmg(opts.Runner, dmg, img); err != nil {
		return nil, "", err
	}
	records := make([]AssetRecord, 0, 3)
	for _, item := range []struct {
		path   string
		source string
	}{
		{dmg, "Apple recovery service"},
		{chunklist, "Apple recovery service"},
		{img, "verified BaseSystem.dmg converted by dmg2img"},
	} {
		record, err := fileRecord(item.path, item.source)
		if err != nil {
			return nil, "", exitcode.New(exitcode.Generic, "cannot verify '%s': %v", item.path, err)
		}
		records = append(records, record)
	}
	return records, product, nil
}

func exists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.Mode().IsRegular() && st.Size() > 0
}

func convertDmg(runner hostcheck.Runner, dmg, img string) error {
	partial := img + ".partial"
	os.Remove(partial)
	defer os.Remove(partial)
	if runner == nil {
		runner = hostcheck.ExecRunner{}
	}
	if err := runner.Run("dmg2img", "-i", dmg, partial); err != nil {
		return exitcode.New(exitcode.Generic, "cannot convert '%s' into a raw disk: %v", dmg, err)
	}
	if !exists(partial) {
		return exitcode.New(exitcode.Generic, "the conversion of '%s' produced an empty image", dmg)
	}
	if err := os.Rename(partial, img); err != nil {
		return exitcode.New(exitcode.Generic, "cannot publish the converted recovery image: %v", err)
	}
	return nil
}

func fetchOpenCore(opts Options) ([]AssetRecord, error) {
	openCoreURL, err := bootAssetURL("opencore")
	if err != nil {
		return nil, err
	}
	firmwareURL, err := bootAssetURL("firmware")
	if err != nil {
		return nil, err
	}
	varsURL, err := bootAssetURL("vars")
	if err != nil {
		return nil, err
	}
	items := []struct {
		name   string
		url    string
		sha256 string
	}{
		{OpenCoreName, openCoreURL, openCoreSHA256},
		{FirmwareName, firmwareURL, firmwareSHA256},
		{VarsName, varsURL, varsSHA256},
	}
	records := make([]AssetRecord, 0, len(items))
	for _, item := range items {
		dest := fileIn(opts.Dir, item.name)
		legacy := filepath.Join(opts.LegacyDir, item.name)
		if sum, err := checksumFile(dest); err != nil || sum != item.sha256 {
			if sum, err := checksumFile(legacy); err == nil && sum == item.sha256 {
				if err := copyFileAtomic(legacy, dest); err != nil {
					return nil, err
				}
			} else {
				describe("fetching the pinned, hash-verified " + item.name)
				if err := downloadFileVerified(item.url, dest, item.sha256); err != nil {
					return nil, err
				}
			}
		}
		record, err := fileRecord(dest, item.url)
		if err != nil || record.SHA256 != item.sha256 {
			return nil, exitcode.New(exitcode.Generic, "the pinned asset '%s' failed SHA-256 verification", item.name)
		}
		records = append(records, record)
	}
	return records, nil
}

func copyFileAtomic(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	partial := dst + ".partial"
	os.Remove(partial)
	out, err := os.OpenFile(partial, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		out.Close()
		if !ok {
			os.Remove(partial)
		}
	}()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	if err := out.Sync(); err != nil {
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	if err := os.Rename(partial, dst); err != nil {
		return err
	}
	ok = true
	return nil
}

func downloadFileVerified(rawURL, dest, expectedSHA256 string) error {
	client := &http.Client{Timeout: 30 * time.Minute}
	resp, err := client.Get(rawURL)
	if err != nil {
		return exitcode.New(exitcode.Generic, "cannot download '%s': %v", rawURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return exitcode.New(exitcode.Generic, "cannot download '%s': %s", rawURL, resp.Status)
	}
	return writeDownloadAtomic(resp.Body, dest, expectedSHA256)
}

func writeDownloadAtomic(body io.Reader, dest, expectedSHA256 string) error {
	partial := dest + ".partial"
	os.Remove(partial)
	out, err := os.OpenFile(partial, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return exitcode.New(exitcode.Generic, "cannot write '%s': %v", dest, err)
	}
	ok := false
	defer func() {
		out.Close()
		if !ok {
			os.Remove(partial)
		}
	}()
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(out, h), body); err != nil {
		return exitcode.New(exitcode.Generic, "cannot write '%s': %v", dest, err)
	}
	if err := out.Sync(); err != nil {
		return exitcode.New(exitcode.Generic, "cannot sync '%s': %v", dest, err)
	}
	if err := out.Close(); err != nil {
		return exitcode.New(exitcode.Generic, "cannot close '%s': %v", dest, err)
	}
	if expectedSHA256 != "" && hex.EncodeToString(h.Sum(nil)) != expectedSHA256 {
		return exitcode.New(exitcode.Generic, "the SHA-256 of '%s' does not match the pinned asset", filepath.Base(dest))
	}
	if st, err := os.Stat(partial); err != nil || st.Size() == 0 {
		return exitcode.New(exitcode.Generic, "the download of '%s' is empty", filepath.Base(dest))
	}
	if err := os.Rename(partial, dest); err != nil {
		return exitcode.New(exitcode.Generic, "cannot publish '%s': %v", dest, err)
	}
	ok = true
	return nil
}

func writeArgs(opts Options) error {
	path := fileIn(opts.Dir, ArgsName)
	content := argsTemplate(opts.CPU)
	if err := config.WriteFileAtomic(path, []byte(content), 0o600); err != nil {
		return exitcode.New(exitcode.Generic, "cannot write '%s': %v", path, err)
	}
	describe(fmt.Sprintf("the args file of the machine: %s", path))
	return nil
}

func LookupCPU(name string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" {
		key = "skylake"
	}
	for _, line := range strings.Split(resources.CPUModels, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 {
			continue
		}
		if parts[0] == key {
			return strings.TrimSpace(parts[1]), nil
		}
	}
	return "", exitcode.New(exitcode.Usage,
		"unknown processor model '%s' (use skylake or host)", name)
}

func argsTemplate(cpuModel string) string {
	return strings.ReplaceAll(resources.ArgsTemplate, "{{CPU_MODEL}}", cpuModel)
}
