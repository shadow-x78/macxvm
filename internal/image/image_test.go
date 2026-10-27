// MacXVM - Image Tests (MIT)
// https://github.com/shadow-x78/macxvm

package image

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shadow-x78/macxvm/internal/argsfile"
)

func TestLookupCPU(t *testing.T) {
	skylake, err := LookupCPU("skylake")
	if err != nil || !contains(skylake, "kvm=on") {
		t.Fatalf("LookupCPU(skylake) = %q, %v", skylake, err)
	}
	host, err := LookupCPU("host")
	if err != nil || !strings.HasPrefix(host, "host,") {
		t.Fatalf("LookupCPU(host) = %q, %v", host, err)
	}
	if _, err := LookupCPU("pentium"); err == nil {
		t.Fatal("LookupCPU accepted an unknown model")
	}
}

func TestLookupRelease(t *testing.T) {
	rel, err := lookupRelease("sequoia")
	if err != nil {
		t.Fatal(err)
	}
	if rel.Board != "Mac-7BA5B2D9E42DDD94" {
		t.Fatalf("the board of sequoia = %q", rel.Board)
	}
	if _, err := lookupRelease("windows"); err == nil {
		t.Fatal("lookupRelease() = nil error, want an error")
	}
	fallback, err := lookupRelease("")
	if err != nil {
		t.Fatal(err)
	}
	if fallback.Name != "sonoma" {
		t.Fatalf("the default release = %q", fallback.Name)
	}
}

func TestArgsTemplateCarriesThePlaceholders(t *testing.T) {
	cpu, err := LookupCPU("skylake")
	if err != nil {
		t.Fatal(err)
	}
	text := argsTemplate(cpu)
	if contains(text, "{{CPU_MODEL}}") {
		t.Fatal("the CPU model placeholder was not replaced")
	}
	for _, want := range []string{
		"{{CPUS}}", "{{RAM_MIB}}", "{{DISK_IMG}}", "{{NETDEV}}", "{{MAC}}", "{{ASSETS}}",
		"#install -drive id=InstallMedia",
		OpenCoreName,
		InstallerName,
	} {
		if !contains(text, want) {
			t.Errorf("the args file misses %q", want)
		}
	}
	if !contains(text, "-accel kvm") {
		t.Error("the args file does not name the accelerator")
	}
	if !contains(text, "kvm-pit.lost_tick_policy=discard") {
		t.Error("the args file does not fix the timer of the guest")
	}
	if !contains(text, "cores={{CORES}}") {
		t.Error("the args file does not name the shape of the processors")
	}
	if !contains(text, "netdev=net0") {
		t.Error("the args file does not attach the network card")
	}
}

func TestVerifyChunkList(t *testing.T) {
	dir := t.TempDir()
	payload := []byte("the payload of the image")
	sum := sha256.Sum256(payload)
	chunklist := make([]byte, 0x24)
	copy(chunklist[:4], "CNKL")
	binary.LittleEndian.PutUint32(chunklist[4:8], 0x24)
	chunklist[8] = 1
	chunklist[9] = 1
	chunklist[10] = 2
	binary.LittleEndian.PutUint64(chunklist[12:20], 1)
	binary.LittleEndian.PutUint64(chunklist[20:28], 0x24)
	binary.LittleEndian.PutUint64(chunklist[28:36], 0x24+36)
	entry := make([]byte, 36)
	binary.LittleEndian.PutUint32(entry[:4], uint32(len(payload)))
	copy(entry[4:36], sum[:])
	chunklist = append(chunklist, entry...)
	digest := sha256.Sum256(chunklist)
	unsigned := append(append([]byte{}, chunklist...), digest[:]...)
	imagePath := filepath.Join(dir, "image.bin")
	chunkPath := filepath.Join(dir, "image.chunklist")
	if err := os.WriteFile(imagePath, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(chunkPath, unsigned, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyChunkList(chunkPath, imagePath); err == nil {
		t.Fatal("digest-only chunklist was accepted as signed")
	}

	chunklist[10] = 1
	signature := make([]byte, 256)
	signature[0] = 1
	chunklist = append(chunklist, signature...)
	if err := os.WriteFile(chunkPath, chunklist, 0o600); err != nil {
		t.Fatal(err)
	}
	original := appleSignatureVerifier
	appleSignatureVerifier = func(digest, signature []byte) error {
		if len(signature) != 256 || signature[0] != 1 {
			return errors.New("invalid synthetic signature")
		}
		return nil
	}
	defer func() { appleSignatureVerifier = original }()
	if err := verifyChunkList(chunkPath, imagePath); err != nil {
		t.Fatalf("RSA-signed chunklist = %v", err)
	}
	broken := append([]byte{}, chunklist...)
	broken[0x24+4] ^= 0xff
	if err := os.WriteFile(chunkPath, broken, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyChunkList(chunkPath, imagePath); err == nil {
		t.Fatal("tampered chunk was accepted")
	}
}

func TestVerifyChunkListRejectsTruncatedLayout(t *testing.T) {
	dir := t.TempDir()
	raw := make([]byte, 0x24+36)
	copy(raw[:4], "CNKL")
	binary.LittleEndian.PutUint32(raw[4:8], 0x24)
	raw[8], raw[9], raw[10] = 1, 1, 1
	binary.LittleEndian.PutUint64(raw[12:20], 100)
	binary.LittleEndian.PutUint64(raw[20:28], 0x24)
	binary.LittleEndian.PutUint64(raw[28:36], 0x24+36)
	path := filepath.Join(dir, "bad.chunklist")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyChunkList(path, path); err == nil {
		t.Fatal("truncated chunklist was accepted")
	}
}

func TestVerifyChunkListRejectsAMagicNumber(t *testing.T) {
	dir := t.TempDir()
	chunkPath := filepath.Join(dir, "image.chunklist")
	if err := os.WriteFile(chunkPath, []byte("XXXX000000000000000000000000000000000000000000"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyChunkList(chunkPath, chunkPath); err == nil {
		t.Fatal("verifyChunkList() = nil error, want an error")
	}
}

func TestRandomID(t *testing.T) {
	a := randomID(16)
	b := randomID(16)
	if len(a) != 16 || len(b) != 16 {
		t.Fatalf("the length of the id = %d", len(a))
	}
	if a == b {
		t.Fatal("two identifiers are the same")
	}
	for _, c := range a {
		if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'F')) {
			t.Fatalf("the identifier holds '%c'", c)
		}
	}
}

func TestLinks(t *testing.T) {
	links, err := Links("sonoma")
	if err != nil {
		t.Fatal(err)
	}
	if len(links) < 4 {
		t.Fatalf("Links() returned %d links", len(links))
	}
	for _, l := range links {
		if l.What == "" || l.From == "" {
			t.Fatalf("the link %+v misses a text", l)
		}
	}
}

func TestThePublicKeyOfAppleIsComplete(t *testing.T) {
	if len(applePublicKey) != 256 {
		t.Fatalf("the key of Apple is %d bytes, want 256", len(applePublicKey))
	}
}

func contains(text, want string) bool {
	return len(text) >= len(want) && indexOf(text, want) >= 0
}

func indexOf(text, want string) int {
	for i := 0; i+len(want) <= len(text); i++ {
		if text[i:i+len(want)] == want {
			return i
		}
	}
	return -1
}

func TestVerifySignature(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte("the payload of the image"))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	le := make([]byte, len(sig))
	for i := range sig {
		le[len(sig)-1-i] = sig[i]
	}
	modulus := key.PublicKey.N
	if err := verifySignature(digest[:], le, modulus); err != nil {
		t.Fatalf("verifySignature() = %v", err)
	}
	broken := append([]byte{}, le...)
	broken[10] ^= 0xff
	if err := verifySignature(digest[:], broken, modulus); err == nil {
		t.Fatal("verifySignature() = nil error, want an error")
	}
	other := sha256.Sum256([]byte("another payload"))
	if err := verifySignature(other[:], le, modulus); err == nil {
		t.Fatal("verifySignature() = nil error, want an error")
	}
}

func TestWriteGPT(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "disk.raw")
	const sectors = 6144
	if err := os.WriteFile(path, make([]byte, sectors*sectorSize), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeGPT(path); err != nil {
		t.Fatalf("writeGPT() = %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if raw[0x1FE] != 0x55 || raw[0x1FF] != 0xAA {
		t.Fatal("the protective MBR has no signature")
	}
	if raw[0x1BE+4] != 0xEE {
		t.Fatalf("the type of the protective MBR = %#x, want 0xEE", raw[0x1BE+4])
	}
	if string(raw[512:520]) != "EFI PART" {
		t.Fatal("the primary header has no signature")
	}
	header := raw[512 : 512+gptHeaderLen]
	if binary.LittleEndian.Uint32(header[12:16]) != gptHeaderLen {
		t.Fatal("the size of the header is wrong")
	}
	stored := binary.LittleEndian.Uint32(header[16:20])
	copyHeader := append([]byte{}, header...)
	binary.LittleEndian.PutUint32(copyHeader[16:20], 0)
	if crc32.ChecksumIEEE(copyHeader) != stored {
		t.Fatal("the CRC of the header does not match")
	}
	entries := raw[1024 : 1024+gptEntries*gptEntrySize]
	if crc32.ChecksumIEEE(entries) != binary.LittleEndian.Uint32(header[88:92]) {
		t.Fatal("the CRC of the entries does not match")
	}
	first := binary.LittleEndian.Uint64(entries[32:40])
	last := binary.LittleEndian.Uint64(entries[40:48])
	if first != 2048 || last != (sectors-34)/2048*2048-1 {
		t.Fatalf("the partition is %d..%d, want 2048..%d", first, last, (sectors-34)/2048*2048-1)
	}
	if string(raw[(sectors-1)*sectorSize:(sectors-1)*sectorSize+8]) != "EFI PART" {
		t.Fatal("the backup header has no signature")
	}
}

func TestArgsTemplateIsValid(t *testing.T) {
	cpu, err := LookupCPU("skylake")
	if err != nil {
		t.Fatal(err)
	}
	normal, install, err := argsfile.Parse(argsTemplate(cpu))
	if err != nil {
		t.Fatal(err)
	}
	all := append(append([][]string{}, normal...), install...)
	if err := argsfile.Validate(all); err != nil {
		t.Fatal(err)
	}
	if err := argsfile.CheckPlaceholders(all); err != nil {
		t.Fatal(err)
	}
}
