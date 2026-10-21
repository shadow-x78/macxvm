---
name: Bug report
about: Report broken behaviour or unexpected output
title: "[bug] "
labels: bug
assignees: ''
---

### Environment

- **OS / distro + version**: <!-- e.g. Fedora 44, Ubuntu 24.04, Arch Linux -->
- **Kernel + KVM**: <!-- output of `uname -a` and `ls -l /dev/kvm` -->
- **QEMU version**: <!-- output of `qemu-system-x86_64 --version` -->
- **Go toolchain**: <!-- output of `go version` -->
- **MacXVM version**: <!-- output of `macxvm version` -->
- **Guest**: <!-- the macOS version of the guest, if there is one -->

### Steps to Reproduce

1.
2.
3.

### Expected Behaviour

### Actual Behaviour

### Logs

```text
<paste the output of `macxvm doctor` and the relevant part of ~/.local/share/macxvm/vms/<name>/console.log>
```

### Legal Note

Do **not** attach a disk image, an installer, a configuration file of a guest or
any Apple software. MacXVM does not distribute them and neither should an
issue.
