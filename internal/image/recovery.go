// MacXVM - Recovery Image (MIT)
// https://github.com/shadow-x78/macxvm

package image

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shadow-x78/macxvm/internal/exitcode"
)

const (
	recoveryHost = "osrecovery.apple.com"
	recoveryPath = "/InstallationPayload/RecoveryImage"
	recoveryUser = "InternetRecovery/1.0"
)

type release struct {
	Name   string
	Board  string
	OSType string
}

var releases = []release{
	{"high-sierra", "Mac-7BA5B2D9E42DDD94", "default"},
	{"mojave", "Mac-7BA5B2DFE22DDD8C", "default"},
	{"catalina", "Mac-00BE6ED71E35EB86", "default"},
	{"big-sur", "Mac-2BD1B31983FE1663", "default"},
	{"monterey", "Mac-B809C3757DA9BB8D", "latest"},
	{"ventura", "Mac-4B682C642B45593E", "latest"},
	{"sonoma", "Mac-827FAC58A8FDFA22", "default"},
	{"sequoia", "Mac-7BA5B2D9E42DDD94", "default"},
	{"tahoe", "Mac-CFF7D910A743CAAF", "latest"},
}

func lookupRelease(name string) (release, error) {
	if name == "" {
		name = "sonoma"
	}
	for _, r := range releases {
		if r.Name == name {
			return r, nil
		}
	}
	return release{}, exitcode.New(exitcode.Usage,
		"unknown macOS '%s' (known: %s)", name, strings.Join(releaseNames(), ", "))
}

func releaseNames() []string {
	out := make([]string, 0, len(releases))
	for _, r := range releases {
		out = append(out, r.Name)
	}
	return out
}

type recoveryInfo struct {
	Product    string
	ImageURL   string
	ImageToken string
	ChunkURL   string
	ChunkToken string
	ImageHash  string
	ChunkHash  string
}

func newSession(client *http.Client) (string, error) {
	req, err := http.NewRequest(http.MethodGet, "http://"+recoveryHost+"/", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", recoveryUser)
	req.Close = true
	resp, err := client.Do(req)
	if err != nil {
		return "", exitcode.New(exitcode.Generic, "cannot reach the recovery service of Apple: %v", err)
	}
	defer resp.Body.Close()
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "session" {
			return cookie.Name + "=" + cookie.Value, nil
		}
	}
	return "", exitcode.New(exitcode.Generic, "the recovery service of Apple returned no session")
}

func fetchInfo(client *http.Client, session string, rel release) (recoveryInfo, error) {
	body := strings.Join([]string{
		"cid=" + randomID(16),
		"sn=00000000000000000",
		"bid=" + rel.Board,
		"k=" + randomID(64),
		"fg=" + randomID(64),
		"os=" + rel.OSType,
	}, "\n")

	req, err := http.NewRequest(http.MethodPost, "http://"+recoveryHost+recoveryPath,
		strings.NewReader(body))
	if err != nil {
		return recoveryInfo{}, err
	}
	req.Header.Set("User-Agent", recoveryUser)
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Cookie", session)
	req.Close = true

	resp, err := client.Do(req)
	if err != nil {
		return recoveryInfo{}, exitcode.New(exitcode.Generic,
			"cannot ask the recovery service of Apple for %s: %v", rel.Name, err)
	}
	defer resp.Body.Close()
	answer, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return recoveryInfo{}, exitcode.New(exitcode.Generic, "cannot read the answer of Apple: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		return recoveryInfo{}, exitcode.New(exitcode.Generic,
			"the recovery service of Apple answered %s for %s: %s", resp.Status, rel.Name,
			strings.TrimSpace(string(answer)))
	}
	info := recoveryInfo{}
	for _, line := range strings.Split(string(answer), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), ": ")
		if !ok {
			continue
		}
		switch key {
		case "AP":
			info.Product = value
		case "AU":
			info.ImageURL = value
		case "AT":
			info.ImageToken = value
		case "CU":
			info.ChunkURL = value
		case "CT":
			info.ChunkToken = value
		}
	}
	if info.ImageURL == "" || info.ChunkURL == "" {
		return recoveryInfo{}, exitcode.New(exitcode.Generic,
			"the recovery service of Apple named no image for %s", rel.Name)
	}
	return info, nil
}

func downloadAsset(client *http.Client, rawURL, token, dest string) error {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", recoveryUser)
	req.Header.Set("Cookie", "AssetToken="+token)
	req.Close = true
	resp, err := client.Do(req)
	if err != nil {
		return exitcode.New(exitcode.Generic, "cannot download '%s': %v", filepath.Base(rawURL), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return exitcode.New(exitcode.Generic, "cannot download '%s': %s",
			filepath.Base(rawURL), resp.Status)
	}
	return writeDownloadAtomic(resp.Body, dest, "")
}

func verifyChunkList(chunklistPath, imagePath string) error {
	raw, err := os.ReadFile(chunklistPath)
	if err != nil {
		return exitcode.New(exitcode.Generic, "cannot read the chunk list: %v", err)
	}
	if len(raw) < 0x24 || string(raw[:4]) != "CNKL" {
		return exitcode.New(exitcode.Generic, "the chunk list of Apple is not valid")
	}
	headerSize := binary.LittleEndian.Uint32(raw[4:8])
	fileVersion := raw[8]
	chunkMethod := raw[9]
	signatureMethod := raw[10]
	chunkCount := binary.LittleEndian.Uint64(raw[12:20])
	chunkOffset := binary.LittleEndian.Uint64(raw[20:28])
	signatureOffset := binary.LittleEndian.Uint64(raw[28:36])
	if headerSize != 0x24 || fileVersion != 1 || chunkMethod != 1 {
		return exitcode.New(exitcode.Generic, "the chunk list of Apple has an unknown format")
	}
	if chunkOffset != 0x24 {
		return exitcode.New(exitcode.Generic, "the chunk list of Apple has an unknown layout")
	}
	if signatureMethod != 1 {
		return exitcode.New(exitcode.Generic, "the chunk list of Apple is not RSA-signed; unsigned or unknown signatures are rejected")
	}
	rawLen := uint64(len(raw))
	if chunkOffset > signatureOffset || signatureOffset > rawLen {
		return exitcode.New(exitcode.Generic, "the chunk list of Apple has invalid offsets")
	}
	if signatureOffset+256 > rawLen {
		return exitcode.New(exitcode.Generic, "the RSA signature of the chunk list is truncated")
	}
	if chunkCount > (signatureOffset-chunkOffset)/36 {
		return exitcode.New(exitcode.Generic, "the chunk list of Apple declares too many chunks")
	}
	digest := sha256.New()
	digest.Write(raw[:chunkOffset])

	image, err := os.Open(imagePath)
	if err != nil {
		return exitcode.New(exitcode.Generic, "cannot read the image: %v", err)
	}
	defer image.Close()

	offset := chunkOffset
	for i := uint64(0); i < chunkCount; i++ {
		if offset+36 > signatureOffset || offset+36 > rawLen {
			return exitcode.New(exitcode.Generic, "a chunk entry in the chunk list is truncated")
		}
		entry := raw[offset : offset+36]
		offset += 36
		digest.Write(entry)
		size := binary.LittleEndian.Uint32(entry[:4])
		want := entry[4:36]
		buf := make([]byte, size)
		if _, err := io.ReadFull(image, buf); err != nil {
			return exitcode.New(exitcode.Generic, "the image is shorter than the chunk list")
		}
		sum := sha256.Sum256(buf)
		if string(sum[:]) != string(want) {
			return exitcode.New(exitcode.Generic,
				"the chunk %d of the image does not match the chunk list of Apple", i)
		}
	}
	signature := raw[signatureOffset : signatureOffset+256]
	if err := verifyAppleSignature(digest.Sum(nil), signature); err != nil {
		return err
	}
	return nil
}

var appleSignatureVerifier = verifySignatureWithAppleKey

func verifyAppleSignature(digest, signature []byte) error {
	return appleSignatureVerifier(digest, signature)
}

func verifySignatureWithAppleKey(digest, signature []byte) error {
	return verifySignature(digest, signature, new(big.Int).SetBytes(applePublicKey))
}

func verifySignature(digest, signature []byte, modulus *big.Int) error {
	if len(signature) != (modulus.BitLen()+7)/8 {
		return exitcode.New(exitcode.Generic, "the signature of the chunk list of Apple is too short")
	}
	reversed := make([]byte, len(signature))
	for i := range signature {
		reversed[len(signature)-1-i] = signature[i]
	}
	sig := new(big.Int).SetBytes(reversed)
	computed := new(big.Int).Exp(sig, big.NewInt(65537), modulus)
	expected := pkcs1v15(digest, len(modulus.Bytes()))
	if computed.Cmp(expected) != 0 {
		return exitcode.New(exitcode.Generic,
			"the signature of the chunk list of Apple does not match")
	}
	return nil
}

var digestInfoPrefix = []byte{
	0x30, 0x31, 0x30, 0x0d, 0x06, 0x09, 0x60, 0x86, 0x48, 0x01, 0x65,
	0x03, 0x04, 0x02, 0x01, 0x05, 0x00, 0x04, 0x20,
}

func pkcs1v15(digest []byte, size int) *big.Int {
	padding := size - len(digestInfoPrefix) - len(digest) - 3
	if padding < 8 {
		padding = 8
	}
	block := make([]byte, 0, size)
	block = append(block, 0x00, 0x01)
	block = append(block, bytes.Repeat([]byte{0xFF}, padding)...)
	block = append(block, 0x00)
	block = append(block, digestInfoPrefix...)
	block = append(block, digest...)
	return new(big.Int).SetBytes(block)
}

func randomID(n int) string {
	const alphabet = "0123456789ABCDEF"
	buf := make([]byte, n)
	random := make([]byte, n)
	if _, err := readRandom(random); err != nil {
		for i := range buf {
			buf[i] = alphabet[(i*7+3)%len(alphabet)]
		}
		return string(buf)
	}
	for i := range buf {
		buf[i] = alphabet[int(random[i])%len(alphabet)]
	}
	return string(buf)
}

func readRandom(buf []byte) (int, error) {
	f, err := os.Open("/dev/urandom")
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return f.Read(buf)
}

func newClient() *http.Client {
	return &http.Client{Timeout: 30 * time.Minute}
}

func formatSize(n int64) string {
	return fmt.Sprintf("%.1f GiB", float64(n)/(1024*1024*1024))
}
