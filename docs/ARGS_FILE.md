<div align="center">

# The Args File - MacXVM

[![Version](https://img.shields.io/badge/version-0.6.0-007AFF?style=flat-square&logo=semver)](../CHANGELOG.md)
[![License](https://img.shields.io/badge/license-MIT-1D1D1F?style=flat-square)](../LICENSE)
![Go](https://img.shields.io/badge/go-1.22%2B-00ADD8?style=flat-square&logo=go)
![Platform](https://img.shields.io/badge/platform-Linux%20x86__64-5E5CE6?style=flat-square&logo=linux)

</div>

---

## Language

<a href="ARGS_FILE.md">🇬🇧 English</a> · <a href="ARGS_FILE_AR.md">🇸🇦 العربية</a>

---

## Table of Contents

- [What the Args File Is](#what-it-is)
- [Format](#format)
- [Placeholders](#placeholders)
- [Required Placeholders](#required-placeholders)
- [Forbidden Flags](#forbidden-flags)
- [Injected Flags](#injected-flags)
- [Adapting an Upstream Boot Script](#adapting)
- [Validation and Errors](#validation)
- [Worked Example](#example)

---

<a id="what-it-is"></a>
## What the Args File Is

The args file is the machine definition. You write the QEMU arguments of your guest, MacXVM substitutes a few placeholders, adds the flags it needs to manage the machine and starts QEMU with the result.

MacXVM knows nothing about macOS on QEMU. There is no CPU model, no SMC key, no SMBIOS value, no OpenCore setting and no firmware path anywhere in the code. If your guest needs something, you write it into the args file.

The file is copied into the state directory of the machine when it is registered, and the copy is used from then on.

<a id="format"></a>
## Format

- One or more QEMU arguments per line, with shell-like quoting: `'...'` is literal, `"..."` allows `\"` and `\\`, and a backslash outside quotes escapes the next character.
- Blank lines are ignored.
- A line that starts with `#` is a comment and is ignored.
- A line that starts with exactly `#install ` (one space) is used **only** when the machine is started with `--install`. Use it for the installer medium and the bootloader steps.
- A trailing backslash does not continue a line. Every line stands on its own, and the backslash itself is dropped, so the boot script of another project can be copied as it is.
- The first token of the first line may be the name of the QEMU binary. It is dropped, because MacXVM starts QEMU itself.

```
qemu-system-x86_64 \
  -accel kvm \
  -cpu host \
  -smp {{CPUS}} \
  -m {{RAM_MIB}} \
  -drive file={{DISK_IMG}},format=qcow2,{{DISK_OPTS}} \
  -drive file={{ASSETS}}/efi.img,format=raw \
  -netdev {{NETDEV}} \
  -device virtio-net-pci,netdev=net0,mac={{MAC}}

#install -drive file={{ASSETS}}/installer.img,format=raw
```

<a id="placeholders"></a>
## Placeholders

| Placeholder | Value |
|-------------|-------|
| `{{CPUS}}` | the number of CPUs of the machine |
| `{{CORES}}` | half of the CPUs, at least one |
| `{{RAM_MIB}}` | the memory of the machine in MiB |
| `{{DISK_IMG}}` | the absolute path of the disk of the machine |
| `{{DISK_OPTS}}` | `cache=writeback`, or `cache=unsafe` for a throwaway machine |
| `{{NVRAM}}` | the absolute path of the NVRAM file, only when the machine has one |
| `{{ASSETS}}` | the assets directory of the machine |
| `{{MAC}}` | the MAC address of the machine |
| `{{NETDEV}}` | `user,id=net0,hostfwd=tcp:127.0.0.1:<port>-:22` |

A placeholder is replaced inside any token, so `-smp {{CPUS}}` and `-smp={{CPUS}}` both work. An unknown placeholder is an error, never a silent empty string.

`{{NVRAM}}` and `{{ASSETS}}` fail when the machine has no NVRAM file or no assets directory. Create the machine with `--nvram-template` and `--assets` if you need them.

The network of the guest needs two lines that belong together. The `-netdev` line builds the network of QEMU with the name `net0`, and the `-device` line must point at that name with `netdev=net0`. Without it the card stays outside the network, QEMU writes `netdev net0 has no peer` in the console log, and the port of SSH is never forwarded, so every command that needs the guest fails.

The `-accel kvm` line names the accelerator of QEMU. Some builds of QEMU only use KVM when it is named, and a guest that falls back to the software emulation of the processor is far too slow to be usable. `macxvm doctor` tells you when the accelerator has to be named.

<a id="required-placeholders"></a>
## Required Placeholders

Every args file must contain these five:

```
{{DISK_IMG}}  {{NETDEV}}  {{MAC}}  {{RAM_MIB}}  {{CPUS}}
```

A missing one is reported with exit code 8:

```
macxvm: error: args file must contain {{DISK_IMG}} (see README: args file)
```

<a id="forbidden-flags"></a>
## Forbidden Flags

MacXVM injects the following flags itself, so the args file must not contain them:

| Flag | Why |
|------|-----|
| `-name` | MacXVM names the process after the machine, so that it can be recognised |
| `-pidfile` | MacXVM needs the pid of the process to know whether the machine runs |
| `-qmp` | MacXVM manages the machine over QMP |
| `-monitor` | A monitor on the console collides with the output of MacXVM |
| `-display` | MacXVM decides between headless and VNC |
| `-vnc` | MacXVM decides between headless and VNC |
| `-daemonize` | MacXVM starts QEMU in its own session itself |
| `-serial` | A serial console collides with the output of MacXVM |
| `-nographic` | A serial console collides with the output of MacXVM |

```
macxvm: error: args file must not contain '-monitor' (MacXVM adds it itself)
```

Most boot scripts of other projects use `-monitor stdio`. Delete that line.

<a id="injected-flags"></a>
## Injected Flags

After your arguments, MacXVM appends:

```
-name guest=macxvm-<id>,process=macxvm-<id>
-pidfile <state>/vms/<name>/qemu.pid
-qmp unix:<run>/<id>.sock,server=on,wait=off
-display none
```

With `--vnc` the last line becomes `-vnc 127.0.0.1:<n>`, where `<n>` is the first free display number starting at 1. The port of the display is `5900+n` and is printed when the machine starts.

<a id="adapting"></a>
## Adapting an Upstream Boot Script

The boot assets of a macOS guest come from you. Two open-source projects document how to obtain them and how to boot them:

- [OSX-KVM](https://github.com/kholia/OSX-KVM) by kholia
- [ultimate-macOS-KVM](https://github.com/Coopydood/ultimate-macOS-KVM) by Coopydood

MacXVM copies none of their content, downloads no Apple installer and does not automate the installation of macOS. To use their boot script:

1. Follow the guide of the project to get the installer image, the bootloader and the firmware files into a directory of your choice. This directory becomes the `--assets` directory of the machine.
2. Open the boot script of that project and take the QEMU arguments from it.
3. Remove every flag of the [forbidden list](#forbidden-flags), most of all `-monitor stdio`.
4. Replace the hard-coded values by the placeholders: the memory, the number of CPUs, the disk, the MAC address and the network device.
5. Move the lines of the installer medium into a `#install ` line, so that they are only used while you install macOS.
6. Register the machine with `macxvm create NAME --args-file ./args.txt --assets ./assets`.

<a id="validation"></a>
## Validation and Errors

The args file is validated twice: when the machine is registered and when it starts. Both times the same rules apply.

| Problem | Exit code | Message |
|---------|-----------|---------|
| a quote is never closed | 8 | `args file: unterminated quote (see README: args file)` |
| a forbidden flag | 8 | `args file must not contain '-monitor' (MacXVM adds it itself)` |
| a required placeholder is missing | 8 | `args file must contain {{MAC}} (see README: args file)` |
| an unknown placeholder | 8 | `args file: unknown placeholder '{{NOPE}}' (see README: args file)` |
| `{{NVRAM}}` without an NVRAM file | 8 | `args file: {{NVRAM}} requires a VM with an NVRAM file (create with --nvram-template) (see README: args file)` |
| `{{ASSETS}}` without an assets directory | 8 | `args file: {{ASSETS}} requires --assets DIR (create with --assets) (see README: args file)` |

A machine that fails the validation is never created: the half-built directory is removed again.

<a id="example"></a>
## Worked Example

```bash
macxvm create demo \
  --args-file ./args.txt \
  --assets ./assets \
  --profile xcode \
  --cpus 4 --ram 8G --disk 100 \
  --user admin \
  --nvram-template ./assets/efi-vars.fd \
  --macos-version 14.7
```

```
Created 'demo' (profile xcode, 4 CPUs, 8192 MiB RAM, 100 GiB disk, ssh 127.0.0.1:34851)
```

The machine is now registered. `macxvm start demo --install --vnc` starts it with the installation lines of the args file and opens a VNC display, so that you can install macOS.
