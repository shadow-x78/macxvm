// MacXVM - Asset Fetch (MIT)
// https://github.com/shadow-x78/macxvm

package image

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/hostcheck"
)

const (
	openCoreURL = "https://raw.githubusercontent.com/kholia/OSX-KVM/master/OpenCore/OpenCore.qcow2"
	firmwareURL = "https://raw.githubusercontent.com/kholia/OSX-KVM/master/OVMF_CODE_4M.fd"
	varsURL     = "https://raw.githubusercontent.com/kholia/OSX-KVM/master/OVMF_VARS-1920x1080.fd"
)

const (
	FirmwareName = "OVMF_CODE_4M.fd"
	VarsName     = "OVMF_VARS-1920x1080.fd"
)

func fetchInstaller(opts Options) error {
	rel, err := lookupRelease(opts.OS)
	if err != nil {
		return err
	}
	img := fileIn(opts.Dir, InstallerName)
	if st, err := os.Stat(img); err == nil && st.Size() > 0 {
		describe(fmt.Sprintf("the installer of %s is already there (%s)",
			rel.Name, formatSize(st.Size())))
		return nil
	}

	client := newClient()
	dmg := fileIn(opts.Dir, "BaseSystem.dmg")
	chunklist := fileIn(opts.Dir, "BaseSystem.chunklist")

	if !exists(dmg) || !exists(chunklist) {
		session, err := newSession(client)
		if err != nil {
			return err
		}
		info, err := fetchInfo(client, session, rel)
		if err != nil {
			return err
		}
		describe(fmt.Sprintf("the recovery image of %s from Apple: %s", info.Product, info.ImageURL))
		if err := downloadAsset(client, info.ChunkURL, info.ChunkToken, chunklist); err != nil {
			return err
		}
		if err := downloadAsset(client, info.ImageURL, info.ImageToken, dmg); err != nil {
			return err
		}
	} else {
		describe("the recovery image of Apple is already there")
	}

	describe("the signature and the chunks of the image are checked")
	if err := verifyChunkList(chunklist, dmg); err != nil {
		return err
	}
	describe("the image is converted into a raw disk")
	if err := convertDmg(opts.Runner, dmg, img); err != nil {
		return err
	}
	return nil
}

func exists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.Size() > 0
}

func convertDmg(runner hostcheck.Runner, dmg, img string) error {
	cmd := exec.Command("dmg2img", "-i", dmg, img)
	cmd.Stdout = io.Discard
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return exitcode.New(exitcode.Generic, "cannot convert '%s' into a raw disk: %v", dmg, err)
	}
	return nil
}

func fetchOpenCore(opts Options) error {
	dest := fileIn(opts.Dir, OpenCoreName)
	if st, err := os.Stat(dest); err == nil && st.Size() > 0 {
		describe(fmt.Sprintf("the OpenCore boot loader is already there (%s)", formatSize(st.Size())))
	} else {
		describe("the OpenCore boot loader from the community")
		if err := downloadFile(openCoreURL, dest); err != nil {
			return err
		}
	}
	firmware := fileIn(opts.Dir, FirmwareName)
	if !exists(firmware) {
		describe("the UEFI firmware of QEMU")
		if err := downloadFile(firmwareURL, firmware); err != nil {
			return err
		}
	}
	vars := fileIn(opts.Dir, VarsName)
	if !exists(vars) {
		describe("the variables of the UEFI firmware")
		if err := downloadFile(varsURL, vars); err != nil {
			return err
		}
	}
	return nil
}

func downloadFile(rawURL, dest string) error {
	resp, err := http.Get(rawURL)
	if err != nil {
		return exitcode.New(exitcode.Generic, "cannot download '%s': %v", rawURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return exitcode.New(exitcode.Generic, "cannot download '%s': %s", rawURL, resp.Status)
	}
	out, err := os.Create(dest)
	if err != nil {
		return exitcode.New(exitcode.Generic, "cannot write '%s': %v", dest, err)
	}
	defer out.Close()
	if _, err := io.Copy(out, resp.Body); err != nil {
		return exitcode.New(exitcode.Generic, "cannot write '%s': %v", dest, err)
	}
	return nil
}

func writeArgs(opts Options) error {
	path := fileIn(opts.Dir, ArgsName)
	content := argsTemplate(opts.CPU)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return exitcode.New(exitcode.Generic, "cannot write '%s': %v", path, err)
	}
	describe(fmt.Sprintf("the args file of the machine: %s", path))
	return nil
}

const (
	CPUSkylake = "Skylake-Client,-hle,-rtm,kvm=on,vendor=GenuineIntel," +
		"+invtsc,vmware-cpuid-freq=on,+ssse3,+sse4.2,+popcnt,+avx,+aes,+xsave,+xsaveopt,check"
	CPUHost = "host,-hypervisor,kvm=on,vendor=GenuineIntel," +
		"+invtsc,vmware-cpuid-freq=on,+ssse3,+sse4.2,+popcnt,+avx,+aes,+xsave,+xsaveopt,check"
)

func LookupCPU(name string) (string, error) {
	switch name {
	case "", "skylake":
		return CPUSkylake, nil
	case "host":
		return CPUHost, nil
	}
	return "", exitcode.New(exitcode.Usage,
		"unknown processor model '%s' (use skylake or host)", name)
}

func argsTemplate(cpuModel string) string {
	var b strings.Builder
	b.WriteString("qemu-system-x86_64 \\\n")
	b.WriteString("  -accel kvm \\\n")
	b.WriteString("  -machine q35 \\\n")
	b.WriteString("  -cpu " + cpuModel + " \\\n")
	b.WriteString("  -smp {{CPUS}},cores={{CORES}},sockets=1 \\\n")
	b.WriteString("  -m {{RAM_MIB}} \\\n")
	b.WriteString("  -rtc base=utc,clock=host \\\n")
	b.WriteString("  -global ICH9-LPC.acpi-pci-hotplug-with-bridge-support=off \\\n")
	b.WriteString("  -global kvm-pit.lost_tick_policy=discard \\\n")
	b.WriteString("  -device isa-applesmc,osk=ourhardworkbythesewordsguardedpleasedontsteal(c)AppleComputerInc \\\n")
	b.WriteString("  -smbios type=2 \\\n")
	b.WriteString("  -drive if=pflash,format=raw,readonly=on,file={{ASSETS}}/" + FirmwareName + " \\\n")
	b.WriteString("  -drive if=pflash,format=raw,file={{ASSETS}}/" + VarsName + " \\\n")
	b.WriteString("  -device ich9-intel-hda -device hda-duplex \\\n")
	b.WriteString("  -device qemu-xhci,id=xhci \\\n")
	b.WriteString("  -device usb-kbd,bus=xhci.0 \\\n")
	b.WriteString("  -device usb-tablet,bus=xhci.0 \\\n")
	b.WriteString("  -device vmware-svga \\\n")
	b.WriteString("  -device ich9-ahci,id=sata \\\n")
	b.WriteString("  -drive id=OpenCoreBoot,if=none,snapshot=on,format=qcow2,file={{ASSETS}}/" + OpenCoreName + " \\\n")
	b.WriteString("  -device ide-hd,bus=sata.2,drive=OpenCoreBoot \\\n")
	b.WriteString("#install -drive id=InstallMedia,if=none,file={{ASSETS}}/" + InstallerName + ",format=raw\n")
	b.WriteString("#install -device ide-hd,bus=sata.3,drive=InstallMedia\n")
	b.WriteString("  -drive id=MacHDD,if=none,file={{DISK_IMG}},format=qcow2 \\\n")
	b.WriteString("  -device ide-hd,bus=sata.4,drive=MacHDD \\\n")
	b.WriteString("  -netdev {{NETDEV}} \\\n")
	b.WriteString("  -device virtio-net-pci,netdev=net0,mac={{MAC}}\n")
	return b.String()
}
