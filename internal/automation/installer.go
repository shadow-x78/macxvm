// MacXVM - Automated macOS Installation (MIT)
// https://github.com/shadow-x78/macxvm

package automation

import (
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/shadow-x78/macxvm/internal/config"
)

var ErrLicense = errors.New("Apple license acceptance requires explicit authorization")

type Installer struct {
	Session       *Session
	VM            *config.VMConfig
	AcceptLicense bool
	Log           io.Writer
	Timeout       time.Duration
	AfterErase    func() error
}

func (i *Installer) Run() error {
	if i.Session == nil || i.VM == nil {
		return errors.New("installer automation is not configured")
	}
	if i.Timeout <= 0 {
		i.Timeout = 4 * time.Hour
	}
	if i.Log == nil {
		i.Log = io.Discard
	}
	i.logf("inspecting the guest screen")
	if screen, err := i.Session.Capture(); err == nil {
		switch {
		case isSetupWelcome(screen):
			i.logf("already at the first Setup Assistant screen")
			return nil
		case screen.Has("remaining") || screen.Has("Installing macOS") || screen.Has("Preparing"):
			i.logf("installation is already running; waiting for Setup Assistant")
			return i.waitForSetup()
		}
	}
	i.logf("waiting for macOS Recovery")
	if err := i.bootToRecovery(); err != nil {
		return err
	}
	i.logf("opening Terminal in Recovery")
	if err := i.openTerminal(); err != nil {
		return err
	}
	if err := i.eraseMatchedDisk(); err != nil {
		return err
	}
	if i.AfterErase != nil {
		if err := i.AfterErase(); err != nil {
			return err
		}
	}
	if err := i.Session.QuitTerminal(); err != nil {
		return err
	}
	i.logf("starting the macOS installer")
	if err := i.startInstaller(); err != nil {
		return err
	}
	i.logf("waiting for the first Setup Assistant screen")
	return i.waitForSetup()
}

func (i *Installer) bootToRecovery() error {
	deadline := time.Now().Add(30 * time.Minute)
	for {
		screen, err := i.Session.Capture()
		if err == nil {
			switch {
			case screen.Has("OpenCore Boot Menu"):
				if screen.Has("Base System") {
					_ = i.Session.ClickPhrase("Base System")
				} else if screen.Has("macOS Base System") {
					_ = i.Session.ClickPhrase("macOS Base System")
				} else if screen.Has("macos") {
					_ = i.Session.ClickPhrase("macos")
				} else {
					return fmt.Errorf("OpenCore menu has no recognisable installer entry: %s", trim(screen.Text, 300))
				}
			case screen.Has("Reinstall macOS") && screen.Has("Disk Utility"):
				return nil
			case screen.Has("macOS Utilities"):
				return nil
			case screen.Has("bash-3.2#"), screen.Has("sh-3.2#"), screen.Has("root#"):
				return nil
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("macOS Recovery did not appear; last screen: %s", trim(i.Session.last.Text, 300))
		}
		time.Sleep(4 * time.Second)
	}
}

func (i *Installer) openTerminal() error {
	if screen, err := i.Session.Capture(); err == nil {
		if screen.Has("bash-3.2#") || screen.Has("sh-3.2#") || screen.Has("root#") {
			return i.Session.FocusCenter()
		}
	}
	deadline := time.Now().Add(2 * time.Minute)
	for {
		if i.Session.TryClick("Utilities") {
			time.Sleep(time.Second)
			if i.Session.TryClick("Terminal") {
				if _, _, err := i.Session.WaitFor([]string{"bash-3.2#", "sh-3.2#", "root#"}, 30*time.Second); err == nil {
					return i.Session.FocusCenter()
				}
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("cannot open Terminal in Recovery; last screen: %s", trim(i.Session.last.Text, 300))
		}
		time.Sleep(2 * time.Second)
	}
}

func (i *Installer) eraseMatchedDisk() error {
	i.logf("selecting and erasing only the internal disk matching %d GiB", i.VM.DiskGiB)
	if err := i.Session.Type("sh " + SeedScript); err != nil {
		return err
	}
	if err := i.Session.Enter(); err != nil {
		return err
	}
	_, screen, err := i.Session.WaitFor([]string{
		"RESULT 777", "RESULT 111", "RESULT 222", "RESULT 333",
		"No such file", "not found",
	}, 3*time.Minute)
	if err != nil {
		return fmt.Errorf("disk erase did not report a result: %w", err)
	}
	switch {
	case screen.Has("RESULT 777"):
		i.logf("internal disk erased and formatted APFS")
		return nil
	case seedMissing(screen):
		return fmt.Errorf("installer seed is not mounted at %s", SeedScript)
	case screen.Has("RESULT 222"):
		return fmt.Errorf("no internal disk matches the expected %d GiB; nothing was erased", i.VM.DiskGiB)
	case screen.Has("RESULT 333"):
		return fmt.Errorf("more than one internal disk matches %d GiB; nothing was erased", i.VM.DiskGiB)
	default:
		return fmt.Errorf("diskutil eraseDisk failed; nothing else was attempted")
	}
}

func (i *Installer) command(command string) error {
	if err := i.Session.Type(command); err != nil {
		return err
	}
	if err := i.Session.Enter(); err != nil {
		return err
	}
	time.Sleep(3 * time.Second)
	return nil
}

func (i *Installer) startInstaller() error {
	// The Recovery window must be frontmost after quitting Terminal.
	time.Sleep(2 * time.Second)
	if err := i.Session.DoubleClickPhrase("Reinstall macOS"); err != nil {
		return err
	}
	if _, _, err := i.Session.WaitFor([]string{"To set up the installation", "click Continue"}, 60*time.Second); err != nil {
		return fmt.Errorf("the installer did not open: %w", err)
	}
	if err := i.Session.ClickFraction(0.5, 0.658); err != nil {
		return err
	}
	if _, _, err := i.Session.WaitFor([]string{"must agree to the terms", "Agree"}, 2*time.Minute); err != nil {
		return fmt.Errorf("the Apple license screen did not appear: %w", err)
	}
	if !i.AcceptLicense {
		screen, _ := i.Session.Capture()
		return fmt.Errorf("%w; review the license in VNC and rerun with --accept-license only if you agree. Screen: %s", ErrLicense, trim(screen.Text, 500))
	}
	i.logf("accepting the license because --accept-license was given")
	if err := i.Session.ClickFraction(0.523, 0.660); err != nil {
		return err
	}
	if _, _, err := i.Session.WaitFor([]string{"I have read and agree"}, 60*time.Second); err != nil {
		return fmt.Errorf("the license confirmation dialog did not appear: %w", err)
	}
	if err := i.Session.ClickFraction(0.531, 0.482); err != nil {
		return err
	}
	if _, _, err := i.Session.WaitFor([]string{"Select the disk", "MacHDD"}, 2*time.Minute); err != nil {
		return fmt.Errorf("the target disk screen did not appear: %w", err)
	}
	if err := i.Session.ClickFraction(0.409, 0.570); err != nil {
		return err
	}
	time.Sleep(2 * time.Second)
	if err := i.Session.ClickFraction(0.523, 0.660); err != nil {
		return err
	}
	if _, _, err := i.Session.WaitFor([]string{"will be installed on the disk", "MacHDD"}, 90*time.Second); err != nil {
		return fmt.Errorf("the install summary did not appear: %w", err)
	}
	if err := i.Session.ClickFraction(0.523, 0.660); err != nil {
		return err
	}
	if _, _, err := i.Session.WaitFor([]string{"remaining", "Installing", "Preparing"}, 3*time.Minute); err != nil {
		return fmt.Errorf("the installation did not start: %w", err)
	}
	i.logf("installation started; the guest will reboot several times")
	return nil
}

func (i *Installer) waitForSetup() error {
	deadline := time.Now().Add(i.Timeout)
	for {
		screen, err := i.Session.Capture()
		if err == nil {
			if isSetupWelcome(screen) {
				i.logf("reached the first Setup Assistant screen; stopping before the user account")
				return nil
			}
			if screen.Has("OpenCore Boot Menu") {
				switch {
				case screen.Has("macOS Installer"):
					_ = i.Session.ClickPhrase("macOS Installer")
				case screen.Has("macos"):
					_ = i.Session.ClickPhrase("macos")
				case screen.Has("Base System"):
					_ = i.Session.ClickPhrase("Base System")
				}
			} else if isInstallerScreen(screen) {
				switch {
				case screen.Has("Restart"):
					_ = i.Session.ClickPhrase("Restart")
				case screen.Has("Agree") && i.AcceptLicense:
					_ = i.Session.ClickPhrase("Agree")
				case screen.Has("Continue"):
					_ = i.Session.ClickPhrase("Continue")
				}
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("installation did not reach Setup Assistant within %s; last screen: %s",
				i.Timeout, trim(i.Session.last.Text, 500))
		}
		time.Sleep(5 * time.Second)
	}
}

func isSetupWelcome(screen Screen) bool {
	for _, phrase := range []string{
		"Choose Your Country or Region",
		"Select Your Country or Region",
		"Set Up Your Mac",
		"Start Using Your Mac",
		"Welcome to Your New Mac",
	} {
		if screen.Has(phrase) {
			return true
		}
	}
	return false
}

func isInstallerScreen(screen Screen) bool {
	for _, phrase := range []string{
		"Reinstall macOS",
		"Install macOS",
		"macOS Utilities",
		"macOS Installer",
		"Select a Language",
	} {
		if screen.Has(phrase) {
			return true
		}
	}
	return false
}

var diskBlock = regexp.MustCompile(`/dev/disk(\d+)\s+\(internal,\s*physical\)`)
var sizeToken = regexp.MustCompile(`(?i)(\d+(?:[.,]\d+)?)\s*([KMGT]?[B8])`)

// SelectDisk identifies exactly one internal physical disk whose size matches
// the VM's configured qcow2. OCR often loses the decimal point ("68.7" becomes
// "687") or reads GB as 6B, so the comparison accepts those variants while
// still refusing ambiguity. It never guesses among two plausible disks.
func SelectDisk(text string, expectedGiB int) (string, error) {
	if expectedGiB <= 0 {
		return "", errors.New("the VM has no valid disk size")
	}
	matches := diskBlock.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return "", errors.New("no internal physical disk was found")
	}
	expectedGB := float64(expectedGiB) * 1.073741824
	type candidate struct {
		disk string
		size float64
	}
	var candidates []candidate
	for idx, match := range matches {
		start := match[0]
		end := len(text)
		if idx+1 < len(matches) {
			end = matches[idx+1][0]
		}
		block := text[start:end]
		for _, token := range sizeToken.FindAllStringSubmatch(block, -1) {
			value, err := strconv.ParseFloat(strings.ReplaceAll(token[1], ",", "."), 64)
			if err != nil {
				continue
			}
			unit := strings.ToUpper(token[2])
			switch {
			case strings.HasPrefix(unit, "T"):
				value *= 1024
			case strings.HasPrefix(unit, "M"):
				value /= 1024
			case strings.HasPrefix(unit, "K"):
				continue
			}
			if value < 10 {
				continue
			}
			for _, variant := range []float64{value, value / 10, value / 100} {
				if math.Abs(variant-expectedGB) <= expectedGB*0.25 {
					candidates = append(candidates, candidate{disk: "/dev/disk" + text[match[2]:match[3]], size: variant})
					break
				}
			}
		}
	}
	unique := map[string]candidate{}
	for _, item := range candidates {
		unique[item.disk] = item
	}
	if len(unique) == 1 {
		for _, item := range unique {
			return item.disk, nil
		}
	}
	if len(unique) == 0 {
		return "", fmt.Errorf("no internal disk matches the expected %.1f GB", expectedGB)
	}
	names := make([]string, 0, len(unique))
	for _, item := range unique {
		names = append(names, fmt.Sprintf("%s (%.1f GB)", item.disk, item.size))
	}
	return "", fmt.Errorf("more than one internal disk matches; refusing to guess: %s", strings.Join(names, ", "))
}

func (i *Installer) logf(format string, args ...any) {
	fmt.Fprintf(i.Log, "macxvm autoinstall: "+format+"\n", args...)
}

// Resume continues after the disk was erased. It never erases again.
func (i *Installer) Resume() error {
	if i.Session == nil || i.VM == nil {
		return errors.New("installer automation is not configured")
	}
	if i.Timeout <= 0 {
		i.Timeout = 4 * time.Hour
	}
	if i.Log == nil {
		i.Log = io.Discard
	}
	deadline := time.Now().Add(30 * time.Minute)
	for {
		screen, err := i.Session.Capture()
		if err == nil {
			if isSetupWelcome(screen) {
				return nil
			}
			if installationInProgress(screen) {
				return i.waitForSetup()
			}
			if recoveryVisible(screen) {
				if terminalVisible(screen) {
					if err := i.Session.QuitTerminal(); err != nil {
						return err
					}
				}
				if err := i.startInstaller(); err != nil {
					return err
				}
				return i.waitForSetup()
			}
			if screen.Has("OpenCore Boot Menu") {
				switch {
				case screen.Has("macOS Installer"):
					_ = i.Session.ClickPhrase("macOS Installer")
				case screen.Has("macos"):
					_ = i.Session.ClickPhrase("macos")
				case screen.Has("Base System"):
					_ = i.Session.ClickPhrase("Base System")
				}
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("could not resume the installation; last screen: %s", trim(i.Session.last.Text, 500))
		}
		time.Sleep(3 * time.Second)
	}
}

func seedMissing(screen Screen) bool {
	return screen.Has("No such file") || screen.Has("not found")
}

func recoveryVisible(screen Screen) bool {
	return screen.Has("Reinstall macOS") || screen.Has("macOS Utilities")
}

func terminalVisible(screen Screen) bool {
	return screen.Has("bash-3.2#") || screen.Has("sh-3.2#") || screen.Has("root#") || screen.Has("Terminal")
}

func installationInProgress(screen Screen) bool {
	for _, phrase := range []string{"Installing macOS", "Preparing", "remaining", "Time Remaining"} {
		if screen.Has(phrase) {
			return true
		}
	}
	return false
}
