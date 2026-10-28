// MacXVM - Guest Kext Reader (MIT)
// https://github.com/shadow-x78/macxvm

package kextmap

import (
	"fmt"
	"strings"

	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/sshx"
)

var GuestKexts = []string{
	"AppleVirtualGraphics",
	"AppleParavirtGPU",
	"AppleParavirtGPUIOGPUFamily",
	"AppleParavirtIOSurface",
	"AppleVirtIO",
	"AppleVirtIOStorage",
	"AppleVideoToolboxParavirtualization",
	"IONVMeFamily",
}

const marker = "MACXVM_KEXT:"

func GuestCommand() string {
	var sb strings.Builder
	sb.WriteString("for k in")
	for _, kext := range GuestKexts {
		sb.WriteString(" " + kext)
	}
	sb.WriteString("; do echo \"" + marker + "$k\";")
	sb.WriteString("plutil -convert xml1 -o - /System/Library/Extensions/$k.kext/Contents/Info.plist 2>/dev/null;")
	sb.WriteString("done")
	return sb.String()
}

func ParseGuestOutput(out string) (Map, error) {
	var all Map
	segments := strings.Split(out, marker)
	for _, segment := range segments {
		if segment == "" {
			continue
		}
		name, body, found := strings.Cut(segment, "\n")
		if !found {
			continue
		}
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		plist, err := ParsePlistXML([]byte(body))
		if err != nil {
			continue
		}
		part, err := FromKext(name, plist)
		if err != nil {
			continue
		}
		all.Add(part)
	}
	if len(all.Personalities) == 0 {
		return all, fmt.Errorf("the guest did not return any kext personality")
	}
	all.Sort()
	return all, nil
}

func LoadFromGuest(cfg *config.VMConfig) (Map, error) {
	out, err := sshx.Output(cfg, GuestCommand())
	if err != nil {
		return Map{}, err
	}
	return ParseGuestOutput(out)
}
