<div align="center">

# Security Policy - MacXVM

[![Version](https://img.shields.io/badge/version-0.2.0-007AFF?style=flat-square&logo=semver)](CHANGELOG.md)
[![License](https://img.shields.io/badge/license-MIT-1D1D1F?style=flat-square)](LICENSE)
![Go](https://img.shields.io/badge/go-1.22%2B-00ADD8?style=flat-square&logo=go)
![Platform](https://img.shields.io/badge/platform-Linux%20x86__64-5E5CE6?style=flat-square&logo=linux)

</div>

---

## Table of Contents

- [Supported Versions](#supported-versions)
- [Reporting a Vulnerability](#reporting)
- [Disclosure Policy](#disclosure)
- [Security Considerations](#considerations)
- [Trust Boundaries](#boundaries)

---

<a id="supported-versions"></a>
## Supported Versions

| Version | Supported |
|---------|-----------|
| 0.1.x | ✅ |

MacXVM is pre-1.0. Every fix lands on `main` and in the next release.

<a id="reporting"></a>
## Reporting a Vulnerability

Please **do not** open a public issue for a security problem.

Report it privately through the security advisory form of the repository:

<https://github.com/shadow-x78/macxvm/security/advisories/new>

Include the version of MacXVM, the version of QEMU, the output of
`macxvm doctor` and the steps that reproduce the problem. Please do not attach
a disk image, a configuration file or anything from a guest.

You will get an acknowledgement within a week, and a fix or an explanation
when the report has been confirmed.

<a id="disclosure"></a>
## Disclosure Policy

A vulnerability is fixed in a release, and the advisory is published together
with the release notes. The reporter is credited, unless they prefer to stay
anonymous.

<a id="considerations"></a>
## Security Considerations

**The guest runs code from the internet.** A macOS guest in a virtual machine
receives the same updates and the same malware as a physical machine. Keep the
automatic updates of the guest under control, and remember that the slimming
level 1 switches them off on purpose, because a system update can break the
boot of such a guest.

**The QMP socket is not authenticated.** It lives in the run directory, which
MacXVM only uses when it is a real directory of the current user that no other
user may write to. A local user who can reach that socket can control the
guest, so the directory must not be shared.

**The SSH connections do not verify the host key.** The guest is reached
through a loopback port, and the key of the guest changes with every
installation. Every value that comes from the user is quoted for the remote
shell, and the arguments of the commands are always vectors, never a shell
string.

**The paths of a fetch stay inside the project.** A `fetch` list of a cloned
repository may not write anywhere else on the host, so that building a project
of somebody else cannot overwrite a file of yours.

**The name of a stored configuration is checked.** Every path of the state
directory is built from the name that `config.json` holds, so a configuration
whose name is not a valid VM name is refused when it is read. A broken or a
hand-edited file cannot make a delete reach outside of the state directory.

**MacXVM never runs as root.** If a step needs root on the host, the tool
prints the command for you and exits. It never calls `sudo` itself.

**The boot assets are yours.** MacXVM ships no Apple software, no firmware, no
boot loader and no keys, and it never downloads any. The machine definition,
the bootloader, the firmware and the installer image come from you, and you
are responsible for them.

<a id="boundaries"></a>
## Trust Boundaries

| Boundary | Trust |
|----------|-------|
| The args file of the user | Trusted: it is the machine definition, and it is the user who starts it |
| `macxvm.json` of a cloned repository | Untrusted for the paths of a fetch: they may not leave the project |
| The state directory | Private to the user, mode 0700. The name of a configuration is validated when it is read |
| The run directory | Private to the user, checked for ownership and mode |
| The guest | Semi-trusted: it holds the source code of the user and runs the builds |
| The host | Trusted: MacXVM runs with the rights of the user who started it |
