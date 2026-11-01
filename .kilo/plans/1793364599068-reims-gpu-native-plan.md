# خطة تنفيذ MXGPU الشاملة داخل MacXVM — نسخة منقحة للتنفيذ

**التاريخ:** 2026-10-30  
**الغرض:** خطة حرفية لوكيل تنفيذ غير خبير بالمشروع، مع تبعيات وبوابات تمنع ادعاء نجاح غير حقيقي.  
**المنتج:** تمرير RTX 5060 إلى macOS، تعريف NVIDIA/Metal مستقل، مشاركة GPU، وكل ميزات العرض والفيديو والطاقة والترحيل والتوزيع.  
**قاعدة النطاق:** جميع البنود التي استُبعدت سابقًا موجودة كمهام واختبارات قبول. لا يوجد قسم «خارج النطاق».

---

## 0. عقد التنفيذ

1. نفّذ المهام بترتيب التبعيات في القسم 25، لا بترتيب أسماء الملفات أو سهولة المهمة.
2. لكل مهمة: **Depends on**، **Files**، **Steps**، **Gate/Evidence**. لا تبدأ مهمة إذا كان artifact المطلوب من تبعيتها غائبًا أو فاشلًا.
3. نفّذ بوابة القبول في موضعها، لا بعد كل التطوير:
   - G0 بعد clean-room/license baseline.
   - G1 بعد أوضاع VFIO/Reims والاستعادة.
   - G2 بعد MXRM/MXKMS.
   - G3 بعد physical framebuffer وIOSurface.
   - G4 بعد أول Metal حقيقي ثم ميزات Metal الكاملة.
   - G5 بعد Video/HDR/multi-monitor.
   - G6 بعد Sleep/Wake.
   - G7 بعد Migration.
   - G8 بعد Sonoma/Tahoe والترقية.
   - G9 بعد package/sign/install/remove/recovery.
   - G10 بعد CI والتوثيق والاختبارات النهائية.
4. احفظ أدلة كل بوابة في `artifacts/mxgpu/<gate>/` مع hardware IDs وmacOS build وSDK وdriver/firmware/QEMU hashes.
5. لا تستخدم NullMoth كمصدر أو dependency. يستخدم كمرجع سلوكي عام فقط.
6. لا تضمّن macOS أو Apple SDK أو kexts أو firmware أو OpenCore binaries داخل `bin/macxvm` أو Git.
7. حافظ على `go.mod` بلا dependencies جديدة و`bin/macxvm <10 MiB`.
8. أي تغيير لـIOMMU/bootloader/initramfs/SIP/kernel collection يحتاج موافقة صريحة وrollback مُختبرًا.
9. اختبر على VMs منسوخة من أساس مختوم، لا على بيانات المستخدم.
10. لا تقبل triangle أو screenshot كدليل Metal إذا لم يُبنَ تطبيق الاختبار من `.metal` source وينفذ عبر `Metal→MXAIR→NVK→MXRM` المباشر.
11. لا تستبدل اختبارًا فاشلًا بتوثيق أو workaround صامت.
12. أنشئ `artifacts/mxgpu/execution-ledger.json` منذ البداية. لكل task ID سجّل dependencies وstatus وevidence paths وlogs وhashes وسبب الفشل إن وجد. لا تبدأ مهمة إذا كان ledger يثبت dependency ناقصًا، ولا تعلن gate دون evidence مرتبط فيه.

### معنى «نجح»

- الوظيفة تعمل من تطبيق حقيقي.
- لا kernel panic أو WindowServer crash أو GPU fault غير مستعاد.
- توجد logs وmanifests وraw results محفوظة.
- rollback مُختبر.
- الاختبارات القديمة والجديدة ناجحة.

---

## 1. القرارات النهائية

| القرار | القيمة |
|---|---|
| الضيف | macOS 14.8 Sonoma و26.7.1 Tahoe x86_64 |
| العتاد الأول | RTX 5060 / GB206 ووظيفة Audio |
| التعريف | MXGPU مستقل فوق macOS |
| kernel | `MXRM`, `MXKMS`, `MXFB`, `MXAccel`, `MXAGDC` |
| userspace | `MXMTLDriver.bundle`, `MXAIR`, NVK/NAK, `MXVideo` |
| التمرير | VFIO/IOMMU، مالك DMA واحد |
| المشاركة | Reims backend مستقل |
| dedicated | كرت ثانٍ يقود المضيف |
| single-gpu | headless/rebind مع rescue service مستقل |
| العرض | مدخلان للشاشة، أو Reims native window |
| الإدخال | USB controller مخصص أو `usb-host` |
| الموارد | 8 vCPU / 12288 MiB / 100 GiB |
| workload profile | `cli` أو `xcode`، لا يتغير |
| GPU backend | `none`, `vfio`, `reims` |
| DisplayStage | `install`, `daily`, `recovery` |
| OpenCore | صور وهوية مستقلة لكل VM ومرحلة |
| التوزيع | MXGPU package مع SBOM/signature/hashes |
| الترقية | canary VM فقط، ثم موافقة المستخدم |
| recovery | `bochs-display` + VNC + guest boot-arg `-mxoff` |

### قاعدة ملكية GPU

- `vfio`: Linux يفك RTX، وQEMU/macOS يملكه.
- `reims`: Linux/NVIDIA يملك RTX، وQEMU يترجم Metal إلى Vulkan.
- لا VFIO وReims على الجهاز نفسه في الوقت نفسه.
- `single-gpu` نقل ملكية مؤقت، وليس مشاركة DMA.

---

## 2. مكونات MXGPU

| المكوّن | المسؤولية |
|---|---|
| `MXRM.kext` | PCI/BAR/DMA/MSI-X/GSP/VRAM/channels/semaphores/fences/power/reset |
| `MXKMS` داخل MXRM | NVIDIA modeset/NVKMS XNU port، heads/modes/vblank/suspend/resume |
| `MXFB.kext` | IOFramebuffer، EDID/DisplayID، HiDPI/HDR/hotplug/cursor |
| `MXAccel.kext` | IOAccelerator/IOGPU/IOSurface/residency/cache/completion |
| `MXAGDC.kext` | AppleGraphicsDeviceControl policy دون تزوير PCI IDs |
| `MXMTLDriver.bundle` | Metal objects/queues/encoders/resources/fences/argument buffers |
| `MXAIR` | AIR verifier وAIR→SPIR-V lowering |
| `NVK XNU backend` | NVK/NAK فوق MXRM بدل Linux DRM |
| `MXVideo` | NVDEC/NVENC وVideoToolbox/XPC |
| `MXPower` | quiesce/save/standby/resume/restore |
| `MXMigrate` | versioned checkpoint/restore |
| `MXInstaller` | build/sign/install/uninstall/AuxKC/manifest/rollback |
| `MXTests` | ABI/load probes/Metal/golden/stress/power/migration |

---

## 3. بنية المستودع

```text
driver/
  Makefile
  third_party.lock.json
  abi/
    mxrm_abi.h
    mxgpu_versions.h
  research/
    README.md
    pci-gb206.md
    xnu-ioaccelerator.md
    xnu-ioframebuffer.md
    metal-driver-abi.md
    videotoolbox-abi.md
    traces/
  kexts/
    MXRM/
    MXFB/
    MXAccel/
    MXAGDC/
  plugin/
    MXMTLDriver/
  translator/
    MXAIR/
  nvk/
    xnu/
  video/
    MXVideo/
  migration/
  installer/
    build.sh
    package.sh
    sign-release.sh
    install.sh
    uninstall.sh
    verify.sh
    load-dev.sh
  tests/
    abi/
    load-probes/
    metal/
    golden/
    stress/
    power/
    migration/

internal/
  vfio/
  opencore/
  driver/
  migration/

resources/
  args-template-vfio-install.txt
  args-template-vfio-daily.txt
  args-template-vfio-recovery.txt
  args-template-reims-install.txt
  args-template-reims-daily.txt
  args-template-reims-recovery.txt
  opencore-mxgpu/
    config-install.plist
    config-daily.plist
    config-recovery.plist
  opencore-reims/
    config-install.plist
    config-daily.plist
    config-recovery.plist
  mxgpu-release-public-key.txt
```

### قواعد البناء

- `make build`: يبني `bin/macxvm` فقط.
- `make -C driver build`: يبني MXGPU على macOS builder.
- `make -C driver test`: كل driver tests المتاحة على builder.
- `make -C driver test-host`: manifest/SBOM/header tests التي لا تحتاج macOS frameworks.
- `make driver-package`: الهدف الجذري القانوني، ويفوّض إلى `$(MAKE) -C driver package`.
- لا artifacts ثنائية للتعريف في `resources/embed.go`.

---

## 4. نموذج البيانات الدقيق

### 4.1 Schema 2

أضف إلى `VMConfig`:

```text
GPU GPUConfig `json:"gpu"`
CPUModel string `json:"cpuModel,omitempty"`
SSHReady bool `json:"sshReady,omitempty"`
```

المفاتيح القديمة الصحيحة التي يجب الحفاظ عليها:
`schemaVersion`, `profile`, `cpus`, `memoryMiB`, `diskGiB`.

`GPUConfig` وJSON tags:

| Go field | JSON key | Default | Validation |
|---|---|---|---|
| `Backend` | `backend` | `none` عند غياب gpu | none/vfio/reims |
| `Driver` | `driver` | مشتق | none/mxgpu/apple-paravirt |
| `DriverVersion` | `driverVersion` | فارغ | من installed manifest |
| `PCI` | `pci` | فارغ | PCI BDF |
| `AudioPCI` | `audioPci` | فارغ | PCI BDF |
| `HostGPU` | `hostGpu` | فارغ | PCI BDF |
| `ROMFile` | `romFile` | فارغ | regular nonempty file |
| `USBControllerPCI` | `usbControllerPci` | فارغ | PCI BDF |
| `USBDevices` | `usbDevices` | [] | 0..4 |
| `DisplayStage` | `displayStage` | `daily` | install/daily/recovery |
| `MaxHeads` | `maxHeads` | 1 | 1..4 |
| `HDR` | `hdr` | false | ممنوع في production/sealed configs قبل G5؛ مسموح في H2/H3 test VMs للتأهيل |
| `ReBAR` | `rebar` | true | pointer/custom unmarshal للتمييز عن false |
| `SingleGPU` | `singleGpu` | false | rescue matrix required |
| `ReimsRuntime` | `reimsRuntime` | فارغ | directory |
| `ReimsQEMUBinary` | `reimsQemuBinary` | `<runtime>/bin/qemu-system-x86_64` | executable |
| `ReimsResolution` | `reimsResolution` | `1920x1080` | WxH محدود |
| `FirmwareVersion` | `firmwareVersion` | فارغ | manifest match |
| `MigrationVersion` | `migrationVersion` | 1 | checkpoint schema |

`USBHostDevice`:
- `vendorId`, `productId`: أربعة hex digits.
- `bus`, `address`: optional integers.
- لا QEMU strings حرة.

### 4.2 Normalize وmigration

- `none→Driver=none`.
- `vfio` مع Driver فارغ → `mxgpu`.
- `reims` مع Driver فارغ → `apple-paravirt`.
- explicit mismatch مرفوض.
- استخدم `*bool` في DTO أو `UnmarshalJSON` لـReBAR؛ `nil→true` و`false` يبقى false.
- `backend=none` فقط عند غياب `gpu` في JSON القديم.
- ترقية schema 1→2 تُحفظ ذريًا تحت `op.lock`، لا داخل `Load` وحده.
- `CPUModel` يحفظ لاستخدامه عند clone/regeneration.

### 4.3 ملفات VM

```text
<vmdir>/
  config.json
  disk.qcow2
  nvram.fd
  args-install.txt
  args-daily.txt
  args-recovery.txt
  opencore/
    install.qcow2
    daily.qcow2
    recovery.qcow2
    identity.json
    manifest.json
  driver/
    installed-manifest.json
    probe-report.json
    install-backup/
    backup-manifest.json
  migration/
    session.json
    migration-plan.json
    check.json
  console.log
  qemu.pid
  op.lock
```

- `identity.json` mode 0600.
- `args.txt` القديم يبقى لـ`backend=none`.
- clone ينشئ هوية جديدة؛ migration يحافظ على هوية الضيف.

### 4.4 Readiness

الحالات:

```text
created qmp-ready booting ssh-ready manual-setup
driver-installing gpu-probing gpu-ready recovering
migrating-out migrating-in migration-failed
stopping stopped failed
```

القواعد:
- `SSHReady` حقيقة مستقلة.
- SSH readiness لا يخفض `gpu-ready/recovering/migrating-*`.
- `manual-setup` يبقى للتوافق.
- `runstate.State` يعيد `migration-failed` و`recovering` حتى بعد stop.
- `gpu-ready` يتطلب `driver/probe-report.json` ناجحًا، لا transient state فقط.

---

## 5. واجهة الأوامر

### 5.1 الإنشاء

```bash
macxvm create NAME --profile cli --gpu-backend vfio \
  --gpu 0000:01:00.0 --audio 0000:01:00.1 --host-gpu 0000:02:00.0 \
  --cpu host --cpus 8 --ram 12G --disk 100

macxvm create NAME --profile cli --gpu-backend reims \
  --gpu 0000:01:00.0 --reims-runtime PATH \
  --cpu host --cpus 8 --ram 12G --disk 100

macxvm create NAME --profile cli --gpu-backend vfio \
  --gpu 0000:01:00.0 --single-gpu \
  --cpu host --cpus 8 --ram 12G --disk 100
```

- `--disk` رقم GiB، لا `100G`.
- GPU create يولد القوالب الثلاثة داخليًا؛ `--args-file` إلزامي فقط لـ`backend=none` دون base.
- أضف flags: `--gpu-backend --gpu --audio --host-gpu --single-gpu --rom --reims-runtime --reims-resolution --cpu`.
- `dedicated = vfio && !single-gpu` ويتطلب HostGPU.
- `--single-gpu` يرفض HostGPU.
- `--base` يرفض كل GPU flags الجديدة.

### 5.2 المضيف

```bash
macxvm gpu --status --json
macxvm gpu --prepare-vfio --gpu BDF --audio BDF --host-gpu BDF --accept-host-changes
macxvm gpu --prepare-single-gpu --gpu BDF --accept-host-changes
macxvm gpu --unbind-vfio --gpu BDF --restore-host
macxvm gpu --prepare-reims --runtime PATH
```

### 5.3 الجهاز والتعريف

```bash
macxvm start NAME --display install
macxvm start NAME --display daily
macxvm start NAME --display recovery
macxvm start NAME --vnc
macxvm gpu NAME --status
macxvm gpu NAME --install-driver PACKAGE --accept-driver-changes
macxvm gpu NAME --remove-driver
macxvm gpu NAME --recover
macxvm gpu NAME --benchmark --json
macxvm gpu NAME --report --sanitize
macxvm driver verify PACKAGE
macxvm driver info PACKAGE --json
macxvm driver build --builder user@macos-builder
```

### 5.4 Migration

```bash
macxvm migrate NAME --to USER@HOST --check
macxvm migrate NAME --to USER@HOST --live
macxvm migrate NAME --cancel
```

- `--check` read-only report.
- `--live` يعيد compatibility check كاملًا داخليًا قبل أي stop؛ لا يعتمد على check قديم.
- `--cancel` يتطلب `migration/session.json` صالحًا وغير stale.

### 5.5 StartOptions

```text
Stage DisplayStage
VNC bool
Force bool
Incoming string
Env []string
```

- لا `Install bool`؛ يستنتج من `Stage==install`.
- `--install` alias قديم يضبط Stage=install ويرفض تعارض `--display`.
- `open`, `finish`, `ensureRunning`: daily.
- `autoinstall`, `base_bootstrap`: install.
- migration destination: daily + Incoming.

سجّل الأوامر في `usage.go` واختبرها في `cmds_test.go`.

---

## 6. QEMU templates والتوسيع

### 6.1 القوالب

| القالب | المحتوى |
|---|---|
| vfio-install | bochs-display + VNC + BaseSystem readonly + OpenCore install + VFIO اختياري |
| vfio-daily | `-vga none` + VFIO GPU/Audio + OpenCore daily + USB |
| vfio-recovery | bochs-display + VNC + OpenCore recovery؛ لا `-mxoff` هنا |
| reims-install | bochs-display + VNC + BaseSystem + Reims runtime |
| reims-daily | `apple-gfx-pci` + memfd shared + native window |
| reims-recovery | bochs-display + VNC + OpenCore recovery بلا تسريع |

- `-mxoff` guest boot argument داخل OpenCore recovery فقط.
- `CPU_MODEL` create-time-only؛ ممنوع في stored stage files.
- `create`/`image prepare` يولدان الملفات الثلاثة إلى staging وينشرانها ذريًا.
- clone ينسخ ويتحقق من الملفات الثلاثة.
- القيم `64G` و`ResizeGpuBars=13` تجريبية فقط؛ aperture النهائي يشتق من D1 measurements.

### 6.2 placeholders

الحالية:
`CPUS CORES RAM_MIB DISK_IMG DISK_OPTS NVRAM ASSETS MAC NETDEV`.

الجديدة:

```text
OPENCORE_IMG
GPU_PCI GPU_AUDIO_PCI
GPU_ROM_OPT
USB_CONTROLLER_PCI
USB1_VENDOR USB1_PRODUCT USB1_ADDR_OPT
USB2_VENDOR USB2_PRODUCT USB2_ADDR_OPT
USB3_VENDOR USB3_PRODUCT USB3_ADDR_OPT
USB4_VENDOR USB4_PRODUCT USB4_ADDR_OPT
REIMS_ROM
REIMS_RESOLUTION
```

- `GPU_ROM_OPT`: فارغ أو `,romfile=<escaped-path>`.
- `USBn_ADDR_OPT`: فارغ أو `,hostbus=N,hostaddr=N`.
- لا wildcards ولا placeholder يتوسع إلى عدة argv tokens.
- `-incoming` محظور في args-file ويُدار بـStartOptions.

### 6.3 `#when` وBuildStage

حلّل كل سطر إلى:

```text
{Tokens []string, Install bool, When []string}
```

الصيغة:

```text
#when PLACEHOLDER <line>
#install #when PLACEHOLDER <line>
```

- القيمة الفارغة تحذف السطر.
- القيمة غير الفارغة تفعّله وتوسّع داخل token.
- استخدم `BuildStage(text, ctx, stage)` في create/start/clone/validation.
- افحص forbidden flags وunknown placeholders في كل الفروع.
- required placeholders تُفحص على active lines فقط.
- لا empty argv tokens.

### 6.4 مصفوفة العرض والبيئة

| الحالة | QEMU display | البيئة |
|---|---|---|
| none headless | `-display none` | عادية |
| install/recovery | `-display none -vnc 127.0.0.1:N` | VNC إجباري |
| VFIO daily | `-display none` | physical output |
| VFIO rescue VNC | VNC فقط عند وجود HostGPU | rescue صريح |
| Reims native | `-display none` | native window، لا WINDOW=0 |
| Reims VNC/headless | `-display none -vnc ...` | `REIMS_VGPU_WINDOW=0` |
| migration destination | daily + Incoming | migrating-in |

`ReimsQEMUBinary` هو الذي يُسباون في Reims، وليس `cfg.QemuBinary`.

`REIMS_ROM` من مسار runtime موثق، ويُتحقق أنه ملف غير فارغ.

`SpawnWithEnv`:
- يبدأ من `os.Environ()` مع استبدال المفاتيح المكررة.
- allowlist: `REIMS_VGPU_WINDOW`, `REIMS_VGPU_LOG`, `REIMS_VGPU_STATS`, `DISPLAY`, `WAYLAND_DISPLAY`, `XAUTHORITY`, `XDG_RUNTIME_DIR`.
- يحتفظ بـPATH/HOME.
- يرفض `LD_PRELOAD` ومتغيرات الحقن.

---

## 7. المرحلة A/D0: الأساس والتراخيص وCI المبكر

### T-A0 — baseline

**Depends on:** لا شيء.

**Steps:**
1. فرع `feature/mxgpu-full-stack`.
2. شغّل `make test`, `make build`, `./scripts/check-headers.sh`.
3. خذ backup من Sonoma وconfig/args/OpenCore/NVRAM.
4. سجّل Go/QEMU/kernel/NVIDIA versions وحجم binary.
5. أنشئ `artifacts/mxgpu/baseline/`.
6. باستخدام تدفق MacXVM المستقر الحالي قبل أي تعديلات GPU، أنشئ VMَي probes disposable منسوخين من أساسين مختومين: `probe-sonoma-14.8` و`probe-tahoe-26.7.1`، مع SSH وسنابشوت. إن لم توجد قاعدة Tahoe، أنشئها أولًا بالتدفق الحالي.
7. اجعل D2a/D2b/D2c وJ-Dev يستهلكان هذين الـVMين؛ K0/K1 لاحقًا لاختبارات التأهيل الكامل والترقية، وليس لإنشاء أول Tahoe.

**Evidence:** الاختبارات الحالية ناجحة، وكل من Sonoma وTahoe probe VM يقلع وSSH يعمل، ولكل منهما snapshot صالح للاستعادة.

### T-D0 — clean-room وSBOM

**Depends on:** A0.  
**Files:** `driver/research/README.md`, `driver/third_party.lock.json`.

**Steps:**
1. افصل behavioral specs عن implementation.
2. سجّل مصدر كل معلومة: SDK/public docs/trace/experiment.
3. لا تنسخ NullMoth/Apple code.
4. راجع ترخيص كل ملف NVIDIA/Mesa/LLVM على حدة.
5. ثبّت revisions وhashes لكل dependency.
6. أنشئ SBOM/NOTICE/license map.
7. أنشئ trust roots مبكرًا: MXGPU release key، host backup key، وmigration witness key. احفظ private keys root-only خارج Git، وثبّت public keys/fingerprints وkey IDs في config/resources المناسبة مع rotation procedure. لا تولّد مفتاحًا جديدًا لكل package.

**Gate G0:** كل dependency له مصدر وrevision/hash/license، وtrust roots موثقة قبل جلب runtime أو توقيع firmware provenance أو بناء المكونات.

### T-CI0 — CI المبكر

**Depends on:** A0.

**Steps:**
1. أبقِ gofmt/vet/test/build/size.
2. وسّع header checks إلى C/C++/Objective-C/Metal/CMake.
3. استبعد `driver/third_party/**` من header checking وافحصه بـSBOM/lock.
4. استخدم `git ls-files '*.sh'` لـ`bash -n`.
5. أضف `make -C driver test-host`.

**Evidence:** CI يرفض script/header/manifest غير الصالح قبل بدء التعريف.

### T-A1 — Schema وcreate pipeline

**Depends on:** D0, CI0.  
**Files:** `internal/config/*`, `internal/cmds/create.go`, `internal/cmds/init.go`, `internal/cmds/build.go`, `internal/xcode/*`.

**Steps:**
1. نفّذ القسم 4 حرفيًا.
2. افصل workload profile عن GPU backend.
3. نفّذ create flags وvalidation.
4. ولّد القوالب الثلاثة إلى staging.
5. احفظ CPUModel.
6. اجعل `backend=none` يستخدم args.txt القديم دون تغيير.
7. حدّث `create --base` incompatibilities.

**Evidence:** schema 1→2 round-trip، create examples تعمل، وإنشاء Bochs قديم لا يتغير.

### T-A2 — argsfile/start/readiness

**Depends on:** A1.  
**Files:** `argsfile`, `qemu`, `start`, `open`, `finish`, `autoinstall`, `base_bootstrap`, `runstate`.

**Steps:**
1. نفّذ BuildStage و`#when`.
2. نفّذ StartOptions ومصفوفة العرض.
3. نفّذ SpawnWithEnv.
4. حدّث كل startVM callers.
5. اجعل `finish`/seed rewriting يستهدف active stage file.
6. نفّذ SSHReady وtransition precedence.
7. حدّث runstate terminal states.

**Evidence:** لا empty argv، لا `-mxoff` في QEMU argv، و`gpu-ready` لا يعود إلى `ssh-ready`.

---

## 8. المرحلة B: ملكية GPU والمضيف

### T-B0 — discovery وthreat model

**Depends on:** A2.  
**Files:** `internal/vfio/{devices,sysfs,plan}.go`.

**Steps:**
1. عدّد كل PCI function وكل IOMMU-group member؛ لا تدمج groups برمجيًا.
2. اقرأ BDF/vendor/device/class/driver/boot_vga/BAR/reset/driver_override.
3. تحقق من Audio وUSB في groups مستقلة إن وجدت.
4. ارفض group تحتوي host dependency غير ممرّر.
5. أنتج bind/rollback plan JSON.

**Evidence:** fixtures لكل topology، بما فيها Audio في group مختلفة.

### T-B1 — host-wide leases

**Depends on:** B0.  
**Files:** `internal/vfio/lease.go`, `internal/qemu/qemu.go`.

**Steps:**
1. أنشئ lease DB تحت `paths.Run()/leases/` لكل BDF وIOMMU group وUSB controller.
2. VFIO يأخذ `LOCK_EX`؛ Reims `LOCK_SH` بعد اجتياز multi-client test.
3. acquire leases ذريًا بترتيب ثابت يمنع deadlock.
4. مرّر lock FD إلى QEMU عبر `cmd.ExtraFiles` حتى خروجه.
5. أغلق FD في كل pre-spawn failure.
6. stale lease recovery يتحقق من PID/start time/sysfs binding.
7. stop لا يعلن `stopped` قبل QEMU exit وrelease/rebind الناجح.

**Evidence:** لا VMان أو VFIO/Reims يستطيعان امتلاك الجهاز المتعارض؛ delete يرفض release الفاشل.

### T-B2 — dedicated

**Depends on:** B1, C1, C2a.

**Steps:**
1. ثبّت HostGPU على مدخل الشاشة الأول وRTX على الثاني.
2. فعّل IOMMU/Above 4G/ReBAR وعطّل CSM.
3. اطلب driver العامل المناسب لـHostGPU، لا تشترط NVIDIA تحديدًا.
4. عند تطابق PCI IDs استخدم address-based initramfs/udev.
5. لا `blacklist nvidia` إذا كان HostGPU يحتاجه.
6. خذ backups لـgrub/initramfs/udev.
7. طبّق بعد `--accept-host-changes` ولا reboot تلقائيًا.

**Gate G1a:** 20 start/stop/reset cycles بلا فقدان console أو leak lease.

### T-B3 — single-gpu

**Depends on:** B1, C1, C2a.

**Steps:**
1. rescue service مملوك لـservice manager، root-owned persistent state، automatic restart، idempotent recovery.
2. سجّل host driver bindings وdisplay-manager/VT/DRM restoration steps.
3. أوقف compositor/display manager، افصل framebuffer، اربط VFIO.
4. عند stop/failure أعد binding ثم أعد console.
5. لا تفعيل عن بعد قبل اجتياز المصفوفة محليًا.

**اختبارات إلزامية:**
- normal stop.
- QEMU SIGKILL.
- MacXVM SIGKILL.
- guest kernel panic.
- VM timeout.
- initial VFIO bind failure.
- host rebind failure.
- watchdog service restart.

**Gate G1b:** كل حالة تعيد console أو تدخل tested local fail-safe؛ exit code صفر وحده ليس دليلًا.

### T-B4 — Reims shared

**Depends on:** B1, D0, C1, C2a.

**Steps:**
1. ثبّت Reims/QEMU بإصدار وhash في state directory.
2. تحقق من Vulkan 1.2+، `/dev/udmabuf`, memfd, ROM.
3. طبّق `reims-util` على صور C1 المخصصة للـVM مع الحفاظ على identity وinstall/daily/recovery؛ لا تعدّل الأصول المشتركة.
4. اضبط `METAL_ERROR_MODE=5` قبل أول desktop.
5. استخدم ReimsQEMUBinary وReimsResolution.
6. اختبر native window/VNC/headless وmulti-client lease.
7. امنع VFIO على نفس lease.

**Gate G1c:** `system_profiler` يظهر Reims vGPU/Metal وLinux يحتفظ بالكرت.

**G1 = G1a + G1b + G1c + rollback evidence.**

---

## 9. المرحلة C: OpenCore والاستعادة

### T-C0 — plist tooling

**Depends on:** A2.  
**Files:** `internal/opencore/plist.go`, tests.

**Steps:**
1. استخدم `encoding/xml` فقط.
2. ادعم dict/array/string/data/integer/real/true/false.
3. getters/setters لمسارات محددة، لا string replacement.
4. ارفض DTD أو plist غير المتوقع.
5. اكتب round-trip tests.

**Evidence:** لا فقدان أنواع أو مفاتيح.

### T-C1 — identity/generation

**Depends on:** C0.

**Steps:**
1. انسخ immutable source إلى VM staging، لا تعدّل assets المشتركة.
2. ولّد SMBIOS serial/MLB/UUID/ROM فريدًا.
3. ابدأ بـ`iMacPro1,1` وثبّت النموذج بـAGDC probes.
4. ولّد install/daily/recovery.
5. احسب hashes وانشر ذريًا.
6. انسخ الأصول عند clone ثم أعد توليد الهوية والصور.
7. نظّف identity-bearing NVRAM variables وSSH host keys عند clone.

**Evidence:** لا تكرار هوية بين VMs ولا writable OpenCore مشترك.

### T-C2a — stage configurations المقاسة

**Depends on:** A0, C1, D1.

**Steps:**
1. install/recovery: `bochs-display` + VNC.
2. recovery OpenCore فقط يحتوي `-mxoff`.
3. daily: ReBAR/aperture من D1، لا constants عمياء.
4. حدّد IONDRVSupport policy عبر A/B probes على Sonoma/Tahoe.

**Evidence:** configs النهائية تستخدم measured values، وA/B matrix لكل OS build.

### T-C2b — runtime boot handoff

**Depends on:** C2a, E2b.

**Steps:**
1. أثناء RM/GSP/MXKMS init، أمسك display initialization حتى display arm.
2. MXFB يصبح display index 0 فقط بعد readiness.
3. timeout يحرر hold إلى recovery-safe path دون partially initialized display.
4. اختبر delayed GSP وcold boot WindowServer.
5. اختبر failed-driver cold boot إلى recovery.

**Evidence:** boot-handoff log، delayed-GSP test، recovery boot بعد driver failure.

---

## 10. المرحلة D: probes والبرنامج الثابت

### T-D1 — GB206/PCI/GSP measurements

**Depends on:** B1.

**Steps:**
1. وثّق PCI/subsystem IDs.
2. قس BAR0/BAR1 وReBAR effects.
3. وثّق MSI-X/reset/VBIOS/GOP.
4. سجّل GSP/firmware versions/hashes.
5. أنتج aperture recommendations بدل فرض 64G/13.

**Evidence:** repeatable probes بعد reboot/rebind.

### T-D1a — firmware provisioning وprovenance

**Depends on:** D0, D1.

**Steps:**
1. ثبّت NVIDIA RM release وmatching GSP version في lock file.
2. نفّذ download/offline-input provisioning tool.
3. أنشئ signed/pinned firmware provenance bundle يحتوي upstream acquisition evidence، digest/size، RM release وABI compatibility، supported GPU/subsystem targets، وسجل redistribution license. إذا لم يوجد upstream signature، وقّع curated bundle بمفتاح MXGPU release.
4. تحقق من الـbundle قبل الملفات، ثم من SHA256 لكل firmware؛ لا تقبل hash صحيحًا لزوج RM/GSP غير متوافق.
5. ثبّت في guest directory إصدارًا root-owned.
6. E2 يفشل مغلقًا عند absence/mismatch/incompatibility.
7. package verify يفرق بين package وinstalled firmware manifest.
8. انقل RM/GSP identity إلى installed manifests وmigration compatibility وcheckpoint metadata.

**Evidence:** online وoffline provisioning يتحققان من الـbundle نفسه ويعطيان hashes نفسها؛ tampered manifest وincompatible pair مرفوضان.

### T-D2a — minimal IOKit load probes

**Depends on:** D0, A0.

**Steps:**
1. ابنِ minimal kext لكل required IOKit class.
2. حمّله عبر المسار الفعلي على كل macOS build.
3. سجّل OS build UUID/SDK/arch/class selectors/struct layouts/signing/SIP/AMFI/SecureBoot/logs.
4. لا تكتفي بـnm أو notes.

**Evidence:** `driver/tests/load-probes/iokit-<osbuild>.json` ناجح.

### T-D2b — minimal Metal bundle load probe

**Depends on:** D0, A0.

**Steps:**
1. ابنِ minimal Metal bundle.
2. حمّله عبر Metal framework discovery الفعلي.
3. سجّل principal classes/dependencies/entitlements/library validation/AMFI requirements.
4. كرر على Sonoma وTahoe.

**Evidence:** test app يكتشف bundle ويستدعي entry point دون GPU claims.

### T-D2c — minimal VideoToolbox probes

**Depends on:** D0, A0.

**Steps:**
1. ابنِ minimal decode/encode components.
2. استدعِهما من client processes حقيقية.
3. اختبر XPC/service registration وTahoe `VTEncoderXPCService`.
4. سجّل entitlements/signing/sandbox requirements.

**Evidence:** decode/encode test sessions تُنشأ دون GPU implementation.

---

## 11. المرحلة E: MXRM وMXKMS

### T-J-Dev — development signing/load path

**Depends on:** D2a, A0.

**Steps:**
1. reproducible development signing.
2. مصفوفة SIP/AMFI/SecureBoot لكل OS build.
3. `load-dev.sh` للـdevelopment load، وAuxKC cold-boot path منفصل.
4. لا تفترض unload مدعومًا إذا كان loading mechanism لا يدعمه.
5. production package لا يرث development overrides.

**Evidence:** load/reload/cold-boot موثق، وrecovery يعمل.

### T-E0 — MXRM skeleton

**Depends on:** D2a, J-Dev.

**Steps:**
1. Info.plist وPCI personality.
2. probe/start/stop/free.
3. `-mxoff` قبل أي hardware write.
4. PCI memory/bus master/MSI-X عند الجاهزية فقط.

**Evidence:** repeated load/start/stop، و`-mxoff` لا يلمس الجهاز.

### T-E1 — BAR/IRQ/DMA

**Depends on:** E0.

**Steps:**
1. map BAR0/BAR1.
2. 64-bit DMA وMSI-X.
3. IODMACommand/IOMemoryDescriptor helpers.
4. IRQ handler/workloop.
5. cleanup في كل error path.

**Evidence:** MMIO/MSI-X/DMA map-unmap stress.

### T-E2 — XNU OS layer وGSP

**Depends on:** E1, D1a.

**Steps:**
1. locks/threads/workqueues/timers/atomics.
2. physical allocations/user mappings.
3. NVIDIA RM فوق XNU.
4. firmware version/hash enforcement.
5. state machine:

```text
Detached → PCIOwned → BARsMapped → FirmwareLoaded
→ RMReady → ChannelsReady → DisplayReady → Running
→ Quiescing → Sleeping | Resetting | Failed
```

6. cleanup عند كل انتقال فاشل.

**Evidence:** GSP ready و20 init/reset cycles.

### T-E2b — MXKMS

**Depends on:** E2.

**Steps:**
1. license-map NVIDIA nvidia-modeset/NVKMS components.
2. نفّذ XNU OS interface للـmodeset.
3. head discovery/mode setting/vblank.
4. suspend/resume.
5. عرّف MXFB↔MXKMS interface وownership.
6. اختبر head/mode transitions دون MXFB الكامل أولًا.

**Evidence:** physical mode set/vblank و50 suspend/resume low-level cycles.

### T-E3 — memory/channels

**Depends on:** E2.

**Steps:**
1. VRAM/system allocations.
2. GPU page tables وVA mappings.
3. channels/pushbuffers/semaphores/fences.
4. versioned descriptors.
5. watchdog/timeout/reset.

**Evidence:** alloc/map/copy/fence roundtrip وfuzz.

### T-E4 — secure user client

**Depends on:** E3.

**Threat model إلزامي:**
- كل client له GPU VM/address-space context مستقل.
- userspace لا يعطي physical DMA addresses اعتباطية.
- handles scoped وغير قابلة للتخمين، مع client+generation checks.
- IOSurface sharing بحقوق transferable صريحة.
- doorbell slots ثابتة الحجم، indices محدودة، overflow-safe arithmetic، atomic sequence validation.
- لا unchecked userspace pointers.
- GPU privileged methods تُرشّح وتُمنع من clients غير المخولة.
- concurrent reset/teardown لا يعيد استخدام handle قديم.

**اختبارات:**
- cross-client handle guessing.
- stale handles after death.
- DMA خارج mappings.
- malformed rings.
- reset/teardown races.
- client starvation.

**Gate G2:** لا cross-client read/write، وGSP/memory/channels/fences/reset/MXKMS ناجحة.

---

## 12. المرحلة F: العرض وIOSurface

### T-F0 — MXFB

**Depends on:** E2b, C2b.

**Steps:**
1. heads وEDID/DisplayID.
2. modes/timings/color capabilities.
3. IOFramebuffer mode/cursor/vblank.
4. headless EDID/dummy head.
5. hotplug/retraining.
6. no scanout قبل fence completion.

**Evidence:** physical وheadless display دون WindowServer race.

### T-F1 — MXAGDC

**Depends on:** F0.

**Steps:**
1. policy selectors من D2a.
2. IORegistry GPU/PCI/FB/Accel links.
3. لا تغيير PCI IDs.
4. اختبر policies على OS targets.

**Evidence:** `system_profiler` يعرض RTX والشاشات والقدرات الفعلية.

### T-F2 — MXAccel/IOSurface مستقلًا عن NVK

**Depends on:** F1.

**Steps:**
1. surface alloc/free/private/shared.
2. cache flush/invalidate.
3. residency/lifetime.
4. fence/event completion.
5. WindowServer notifications.
6. `mxnoflip` copy path.
7. لا تربط NVK handles بعد؛ استخدم test backend/mock.

**Gate G3:** physical framebuffer وIOSurface ABI/lifetime/WindowServer stability. لا تدّعِ Metal rendering هنا.

---

## 13. المرحلة M: Metal بالترتيب الصحيح

### T-M0 — Metal discovery

**Depends on:** D2b, F2, E4.

**Steps:**
1. MXMTLDriver.bundle وInfo.plist.
2. device enumeration/capabilities الأساسية.
3. اربط device بـMXAccel/MXRM clients.
4. لا تعلن ميزة غير منفذة.

**Evidence:** framework يكتشف device عبر المسار الحقيقي.

### T-M1a — Metal ABI objects مع mock backend

**Depends on:** M0.

**Steps:**
1. device/queue/command buffer objects.
2. buffer/texture/heap/sampler objects.
3. blit/compute/render encoder APIs.
4. fence/event APIs.
5. lifetime/error tests.

**Evidence:** API behavior فقط؛ ممنوع تسمية هذا hardware rendering.

### T-M3a — minimal NVK XNU path

**Depends on:** E4.

**Steps:**
1. عدّد Linux DRM calls المطلوبة.
2. نفّذ XNU backend فوق MXRM.
3. device/channel/BO/VM/fence operations.
4. NAK compiler وcommand generation.
5. أول GPU command path حقيقي.

**Evidence:** NVK init وsimple compute/render submission دون Metal plugin.

### T-F3 — IOSurface/NVK integration

**Depends on:** F2, M3a.

**Steps:**
1. اربط IOSurface resource handles بـNVK BO/VM handles.
2. ثبّت lifetime والملكية عبر MXAccel وNVK.
3. تحقق من cache coherence وfence ordering.
4. نفّذ zero-copy path بعد copy fallback.
5. اختبر teardown/concurrent reset/client death.

**Evidence:** surface hashes وfence traces صحيحة، ولا use-after-free أو stale mapping.

### T-M4a — minimal MXAIR

**Depends on:** M3a.

**Steps:**
1. AIR reader/verifier.
2. types/address spaces/arithmetic/control flow الأساسية.
3. textures/samplers اللازمة للـtriangle.
4. compute vector-add lowering.
5. رفض أي opcode غير مفهوم.

**Evidence:** يترجم فقط shaders المطلوبة للاختبارين من `.metal` source.

### T-M1b — أول Metal حقيقي

**Depends on:** M1a, M4a, F3.

**Steps:**
1. شغّل triangle من Metal app.
2. شغّل compute vector-add.
3. تحقق من pixel/results وGPU execution evidence.
4. امنع prerecorded command buffers أو hardcoded images.

**Gate G4-real:** `Metal→MXAIR→NVK→MXRM` مثبت من source إلى pixels.

### T-M2 — Metal feature completion

**Depends on:** M1b.

**Steps:**
1. blit/compute/render encoders كاملة.
2. resources/heaps/fences/events/shared events.
3. residency/hazards.
4. argument buffers tier 2.
5. descriptor arrays/dynamic indexing/samplers/texture arrays.
6. indirect command buffers.
7. mesh shaders وray tracing بعد capability probes وعلى GB206 المدعوم.

**Evidence:** MPS/Core Image/WebGL/ANGLE/Xcode shader tests.

### T-M3 — NVK completion

**Depends on:** M3a, M1b.

**Steps:**
1. أكمل BO/VM/channel/fence operations.
2. traces للمقارنة مع Linux/NVK.
3. performance counters وerror reporting.
4. stress memory/reset.

### T-M4 — MXAIR completion

**Depends on:** M4a, M1b.

**Steps:**
1. atomics وpointer provenance و64-bit arithmetic.
2. argument buffers/resource metadata.
3. arrays/dynamic samplers/shadow maps.
4. Sonoma/Tahoe AIR variants.
5. errors مع repro bundles؛ لا silent fallback.

**Gate G4:** كل ميزة معلنة لها executable test، وgolden images صحيحة، وbenchmark §20 للـMetal/UI/WindowServer/VRAM/Xcode/NVMe/boot فقط. قياس sleep/wake يتبع G6، وmigration downtime يتبع G7، وكامل mixed-workload stress يتبع G10.

---

## 14. المرحلة H: Video وHDR وتعدد الشاشات

### T-H0 — NVDEC

**Depends on:** D2c, G4.

**Steps:**
1. decode sessions في MXRM.
2. H.264/HEVC ثم AV1 عند تأكيد GB206.
3. IOSurface output/color metadata.
4. VideoToolbox registration.
5. FFmpeg/Safari/QuickTime tests.

**Evidence:** frame/reference comparisons ولا software fallback خفي.

### T-H1 — NVENC

**Depends on:** H0.

**Steps:**
1. encode sessions/rate control.
2. H.264/HEVC/AV1 حسب GB206.
3. Tahoe VTEncoderXPCService registration.
4. لا تعتمد على Metal device داخل client process.
5. minimal entitlements.

**Evidence:** compression session وbitstream صالح وقابل للتشغيل.

### T-H2 — HDR

**Depends on:** G4, F0.

**Steps:**
1. 10-bit scanout.
2. RGB/YUV formats.
3. PQ/HLG EOTF.
4. mastering metadata.
5. ICC/Night Shift/SDR↔HDR.
6. DDC/CI brightness حيث تدعمه الشاشة.

**Evidence:** color patterns/metadata على HDR display.

### T-H3 — multi-monitor/VRR

**Depends on:** H2.

**Steps:**
1. 1–4 heads.
2. mirroring/extending/rotation/HiDPI.
3. hotplug أثناء WindowServer/Metal.
4. DSC/bandwidth matrix.
5. VRR حيث تثبت EDID وGB206 دعمه.

**Gate G5:** Video encode/decode وHDR وmulti-monitor tests ناجحة.

---

## 15. المرحلة I: الطاقة والترحيل

### T-I0 — Sleep/Wake

**Depends on:** G4, E2b.

**Steps:**
1. أوقف submissions وانتظر fences.
2. اختر واختبر أحد مسارَي VRAM:
   - preserve-in-place إذا أثبت GSP دعمه؛ أو
   - copy إلى system memory مخصصة مسبقًا بسعة used VRAM + metadata.
3. copy fallback له owner وintegrity hashes؛ allocation failure يلغي sleep ويستأنف running بأمان.
4. MXKMS suspend وRM standby.
5. PCI/BAR/MSI-X save/restore.
6. GSP/memory/channels/display restore.
7. IOSurface/texture/video hashes بعد كل دورة.
8. `internal/slim` لا يعطّل sleep على VMs الناجحة.

**Gate G6:** 50 دورة مع desktop/compute/video دون فقدان محتوى.

### T-I1 — migration plan/compatibility

**Depends on:** G6, B1.

**Steps:**
1. أنشئ `migration-plan.json` بإصدار صريح. لكل mapping سجّل guest PCI slot/function، stable source/destination identity، destination BDF أو USB serial+physical port path، IOMMU-group membership، ROM/firmware/runtime digests، وlease ID المقترح.
2. لا تشترط تطابق host BDFs أو paths. طابق GB206/subsystem/firmware/driver/QEMU machine/CPU flags/OpenCore/NVRAM.
3. حافظ على guest PCI slots وSMBIOS/NIC identity وdisk serial/WWN/controller/slot. أنشئ destination chain manifest يصف node order/format/virtual size/backing relationships، مع destination-local relative backing paths.
4. عرّف netdev وSSH forwarding mappings. لا تجعل incoming NIC أو forwarding متاحًا خارجيًا قبل commit؛ بعد commit فعّل forwarding وأرسل ARP/IPv6 ND refresh، واعكس ذلك عند rollback.
5. `--check` read-only: يحسب mappings المقترحة ويجري non-locking availability probes، ولا يحجز IDs/ports/leases ولا يغيّر state.
6. `--live` يعيد validation ثم يحجز destination runtime ID/SSHPort وleases ذريًا قبل Incoming. VFIO حصري، وReims shared فقط عند توافق lease. حرّر الحجوزات عند cancel/failure.
7. قبل إيقاف source vCPUs مباشرة، أعد التحقق من mappings وdigests وtopology وlease ownership. تغيّر USB/PCI/ROM أثناء pre-copy يلغي العملية والمصدر يستمر.

**Evidence:** check ينجح مع destination BDFs وpaths مختلفة، واختبارات USB replug/PCI rebind/ROM replacement تفشل دون لمس المصدر.

### T-I2 — checkpoint format/parser

**Depends on:** I1.

Header:

```text
magic=MXGPUCKP1
endianness=little-endian
headerLength
schemaVersion
totalLength
migrationUUID
attempt
migrationPlanDigest
gpuPCIID/subsystemID
firmwareHash
driverVersion/mxrmABI
qemuMachineABI
sectionCount
sections[]={type,version,flags,offset,length,alignment,sha256}
authentication=session-keyed MAC
```

الأقسام:
- PCI/BAR/MSI-X.
- VRAM.
- page tables.
- channels/pushbuffers.
- fences/semaphores/events.
- display/HDR.
- video sessions.
- IOSurface/resource registry.

القواعد:
- MAC هو HMAC-SHA-256 بمفتاح session عشوائي 32-byte يُنشأ أثناء migration handshake ولا يُخزّن في checkpoint. غطِّ بالترتيب `MXGPU-CHECKPOINT-v1\0 || migrationUUID || attempt || migrationPlanDigest || canonicalHeader || sectionTable || padding || payload`. تحقق constant-time قبل أي restore.
- cap counts قبل allocation، size caps لكل section والإجمالي، overflow-safe lengths/alignment.
- ranges محدودة وغير متداخلة، exact total consumption، ولا trailing bytes.
- ارفض duplicate singleton sections وunknown flags وmissing required sections؛ unknown optional sections تُتخطى فقط ضمن الحدود.
- Parse إلى bounded staging representation وتحقق من كل digests/cross-references قبل تغيير driver state.
- تحقق من handle uniqueness وVA ranges وpage-table bounds وresource references وreplay protection.
- hostile fuzz + truncation/overlap/replay/digest mismatch/invalid cross-reference tests.

### T-I3 — VFIO migration capability

**Depends on:** I2.

**Steps:**
1. افحص `VFIO_DEVICE_FEATURE_MIGRATION` وDMA logging.
2. استخدم pre/stop-copy فقط عند دعم الجهاز الفعلي.
3. أضف QMP migrate/query/cancel/blockdev-mirror/query-block-jobs.
4. Incoming على الوجهة فقط.
5. لا تستنتج دعم GeForce من وجود QEMU API.

**Evidence:** capability report لكل جهاز.

### T-I4 — cooperative migration

**Depends on:** I3.

**Steps:**
1. أنشئ cross-host single-writer lease/epoch عبر migration witness مستقل (خدمة MacXVM صغيرة على مضيف ثالث أو حساب witness منفصل) أو storage/network fencing primitive مكافئ. الـwitness يحتفظ بـ`migrationUUID, epoch, owner, state` في journal ذري، ويقبل CAS فقط بشرط epoch السابق وsession token، وتُثبت هويته بمفتاح D0 المثبت. استخدم persistent local deny-start markers؛ lease ملف محلي وحده لا يكفي بين مضيفين.
2. استخدم migration UUID/attempt-scoped state machine مع fsync بعد كل انتقال:
   `prepared → pre-copy → source-stopped → final-state-sent → destination-validated → source-fenced → committed → destination-active`.
3. سجّل `migration/session.json` فيه session ID وcheck hash وstate وowner epoch. commit record هو `committed`؛ cancel مسموح قبله فقط.
4. أثناء RAM pre-copy، أثبت quiescence لكل GPU DMA وguest mutations أو استخدم tested dirty tracking.
5. قبل switchover أعد validation للmappings/digests/topology/leases. عند switchover أوقف vCPUs ثم interrupts وكل DMA-capable engines.
6. التقط final GPU checkpoint مع final RAM/disk deltas. انقل كامل qcow2 backing chain أو independent full mirror وفق chain manifest، مع destination-local relative backing paths؛ لا writable shared base.
7. انقل NVRAM/OpenCore/images/manifests. حافظ على disk serial/WWN/controller/slot وتحقق من chain قبل restore.
8. أبقِ destination NIC/SSH forwarding غير منشور حتى commit. بعد commit فعّل forwarding وأرسل ARP/IPv6 ND refresh؛ عند rollback أوقفه قبل إعادة source networking.
9. استعد وتحقق من destination state دون resume. ارفع epoch وfence المصدر ثم اكتب commit record، وبعدها فقط استأنف destination CPU/GPU وانشر networking.
10. عند الفشل قبل commit: أوقف وfence الوجهة، ثم استأنف مصدرًا كان متوقفًا إذا كانت الملكية مؤكدة. إذا كانت الوجهة أو liveness غير مؤكدة، أبقِ الجانبين stopped واطلب reconciliation؛ لا rollback أعمى.
11. migration يعمل عبر SSH tunnel أو QEMU TLS.
12. اختبر SSH/QMP loss وCLI/host death وreboot عند كل state، خصوصًا بعد final-state-sent وقبل/بعد commit record، وإلغاءً قبل/بعد switchover.

**Gate G7a:** Metal/WindowServer/video يستأنفون، ولا كاتبان للقرص أو duplicate network reachability في كل success/failure/crash window.

### T-I5a — Reims save/load research وimplementation

**Depends on:** I2, B4.

**Steps:**
1. وثّق `VMStateDescription` أو save/load path في pinned Reims/QEMU.
2. وثّق protocol version/queues/shared-memory dirty tracking.
3. عدّد كل Vulkan object الذي يحتاج reconstruction.
4. نفّذ save/load support الناقص في Reims/QEMU.
5. ممنوع serialize raw pointers أو destination-local handles.
6. file descriptors تحتاج transfer mechanism صريحًا.

**Evidence:** load/save unit tests قبل migration الفعلي.

### T-I5 — Reims migration

**Depends on:** I5a, I4 orchestration.

**Steps:**
1. جمّد protocol queues.
2. انقل protocol state/dirty memory.
3. أعد بناء Vulkan resources.
4. استأنف نفس guest session دون desktop reinitialization.
5. تحقق من rendering/video sentinels.

**Gate G7 = G7a + Reims migration + cancel/failure tests قبل وبعد switchover.**

---

## 16. المرحلة J/K: الحزمة والأنظمة

### T-J-Release — production signing

**Depends on:** G4, وكل component مدرج في الحزمة.

**Steps:**
1. وقّع كل kext/bundle/dylib/service.
2. تحقق من signatures.
3. أنشئ production-security matrix لكل OS build: SIP/AMFI/SecureBoot/user approval.
4. لا تعتمد على development AMFI overrides.
5. notarization حيث ينطبق.
6. استخدم release trust root المنشأ في D0؛ لا تولّد مفتاح release جديدًا لكل حزمة، وتحقق من key ID وrotation policy.

**Evidence:** verify على clean VM دون development overrides.

### T-J0 — package وcanonical signature

**Depends on:** J-Release.

**على macOS builder:**
```bash
make -C driver build
make -C driver test
make driver-package
```

**ترتيب التوقيع الإلزامي:**
1. build payloads.
2. hash كل payload بما فيه SBOM/NOTICE/scripts.
3. أنشئ unsigned manifest.
4. canonicalize للـmanifest.
5. وقّع مرة واحدة بـEd25519 وأرفق detached signature.
6. اشتق `SHA256SUMS` من signed entries؛ لا تجعله مصدر trust مستقلًا.

**Canonical contract:**
- package-format version وkey ID داخل الموقّع.
- domain separation: `MXGPU-PACKAGE-v1\0 || canonical_manifest`.
- canonical JSON هو JCS/RFC 8785: duplicate-key rejection، UTF-8 valid، NFC للـpaths، integer numbers فقط في schema الحقول العددية (لا floats ولا exponent forms)، وkey ordering حسب JCS. لا trailing whitespace أو comments.
- لا mutable post-signature fields؛ أي تغيير بعد التوقيع يجعل التوقيع غير صالح.
- reproducibility وbyte-level signature test vectors.

**المحتوى:**
- kexts/bundles/plugins/services.
- `manifest.json`, detached signature, `SBOM.json`, `NOTICE`, `SHA256SUMS`.
- install/uninstall/verify scripts.
- لا macOS/OpenCore/GSP غير القابل لإعادة التوزيع.

### T-J1 — verify وinstall transaction

**Depends on:** J0.

**قبل رفع الحزمة:**
1. `macxvm driver verify` يتحقق من pinned release public key والتوقيع وSBOM/target/hashes.
2. development package يحتاج explicit per-VM trust exception.
3. تحقق من الحزمة قبل الفتح بصورة آمنة ضد races؛ لا تعتمد على ملف قابل للاستبدال في `/tmp`.

**Archive safety:**
- ارفع الحزمة إلى fresh root-owned directory.
- تحقق من التوقيع ثم انسخ الملف إلى immutable root-owned copy واستخدم النسخة نفسها للفحص والفتح؛ لا تنفّذ من upload path.
- استخرج إلى root-only staging مع exclusive create/no-follow، وبلا ownership أو privilege bits من الأرشيف.
- ارفض absolute paths و`..` والروابط الهاربة والملفات الخاصة والتكرار بعد Unicode normalization وcase folding، وتعامل مع PAX/long-name metadata.
- فرض file-count وactual uncompressed size وexpansion-ratio limits أثناء streaming.
- انشر فقط بعد تحقق كل entry؛ عند الفشل يبقى التثبيت السابق دون تغيير.

**التركيب الذري:**
1. اسمح فقط `backend=vfio && driver=mxgpu`، وارفض Sealed.
2. إن كان VM stopped، شغّل `StageRecovery` وانتظر SSH؛ لا تفترض SSH على VM stopped.
3. ارفع الحزمة وافتحها وفق قواعد archive safety.
4. خذ consistency point: APFS snapshot للـsystem/boot artifacts المحددة، ولا تستخدمه لاستعادة بيانات المستخدم أو كامل القرص.
5. انسخ host-side OpenCore images/NVRAM/active args/installed manifest الحالية إلى `driver/install-backup/`.
6. أنشئ host-side backup manifest موقّعًا Ed25519 بمفتاح backup خاص بالمضيف محفوظ root-only، ويغطي paths/sizes/hashes/VM identity/package ID/transaction ID/APFS snapshot ID وkey ID. ثبّت public-key fingerprint في VM config قبل أول استخدام.
7. اربط backup بـmacOS build/OpenCore/driver/AuxKC state وfsync لكل الجانبين.
8. اختبر AuxKC قبل النشر ثم ثبّت بتبديل ذري.
9. إذا commit نجح في جانب واحد فقط، استخدم transaction ID لاستعادة driver/boot artifacts من الجانبين معًا.
10. أعد التشغيل إلى daily وشغّل probes، ثم اكتب installed-manifest وprobe-report.
11. عند الفشل أقلع recovery ونفّذ uninstall من النقطة الموقعة.

**Evidence:** interruption بين guest commit وhost commit وmanifest publication، وcase/Unicode collision وmetadata alias وcompression bomb وdisk-full وupload-path swap tests.

### T-J2 — remove

**Depends on:** J1.

**Steps:**
1. recovery مع `-mxoff`.
2. uninstall script الرسمي.
3. استعد backup أو AuxKC بدون MXGPU.
4. استعد OpenCore.
5. install mode وSSH verification.

**Evidence:** لا ملفات MXGPU متبقية ولا kernel collection مكسور.

### T-J3 — distribution/updates

**Depends on:** J2.

**Steps:**
1. SBOM/NOTICE/hashes/signature لكل release.
2. firmware غير القابل لإعادة التوزيع يُجلب من NVIDIA عبر D1a.
3. canary VM لكل macOS/driver update.
4. ABI probes بعد كل macOS build.
5. لا auto update لأساس مختوم.

### T-K0 — Sonoma

**Depends on:** G4, J1(Tahoe).

**Steps:**
1. استنسخ Sonoma الأساسي إلى `sonoma-mxgpu` وخذ snapshot فورًا قبل probes أو build أو install أو OpenCore/SIP changes.
2. ABI probes لـ14.8 على `probe-sonoma-14.8`.
3. build ضد SDK الصحيح.
4. كرر تدفق `J-Release(Sonoma) → J0(Sonoma) → J1(Sonoma)` بحزمة مستقلة موقعة ومقصودة لنفس OS build.
5. install/test Metal/FB/Video/power/display.

**Evidence:** Sonoma يعمل دون تعديل الأصل، وpackage/install flow خاص بالـSonoma مُثبت؛ remove/recovery يُثبت لاحقًا في J2.

### T-K1 — Sonoma→Tahoe

**Depends on:** K0.

**Steps:**
1. استنسخ إلى `sonoma-to-tahoe` وخذ snapshot كاملًا قبل الترقية.
2. ترقية رسمية إلى 26.7.1.
3. أعد ABI probes على الـbuild الجديد وأعد OpenCore وMXGPU build.
4. كرر تدفق `J-Release(Tahoe) → J0(Tahoe) → J1(Tahoe)` بعد إعادة البناء وقبل اختبارات التطبيق.
5. اختبر SSH/files/apps/Metal/Video/power/display/recovery.
6. أنشئ sealed base جديدًا بعد النجاح.

**Gate G8:** Sonoma وTahoe والترقية وrollback ناجحة، وكل OS build له حزمة موقعة ومثبتة ومختبرة؛ remove/recovery شرطه G9 بعد J2.

### T-K2 — finish/seal

**Depends on:** G8, J1.

**Steps:**
1. أضف driver stages عند الحاجة.
2. لا Bochs/virtual VGA في daily أو sealed GPU VM؛ install/recovery يحتفظان بمسارات §6.1.
3. `baseSeal` نفسه يطلب `probe-report.json` ناجحًا وpackage hash مطابقًا وrecovery test؛ لا تعتمد على finish وحده.
4. لا seal مع failed Metal/recovery.
5. sealed GPU base لا يعمل ولا يُعدّل إلا عبر clone.

---

## 17. المرحلة L: تكامل دورة الحياة

### T-L0 — clone

**Depends on:** C1, K2.

**Steps:**
1. انسخ وأعد التحقق من args-*.txt/OpenCore/NVRAM/driver manifests/assets.
2. احذف PID/QMP/VNC/console/op.lock/migration transient state.
3. ولّد runtime ID/MAC/SSHPort/SMBIOS identity جديدة.
4. أعد توليد OpenCore images/manifests ذريًا.
5. أعد SSH host keys عند أول boot ونظّف identity NVRAM.
6. leases تمنع التشغيل المتعارض، لا clone-time check.

**Evidence:** clone بهوية مختلفة دون تعديل base.

### T-L1 — delete/recovery

**Depends on:** B1, J2.

**Steps:**
1. delete يتطلب stopped وGPU release الناجح.
2. لا يحذف runtimes/assets المشتركة.
3. recovery على base مختوم يتم عبر clone.
4. release/rebind failure يبقي failed ويمنع delete.

### T-L2 — status/doctor

**Depends on:** G4, J1.

**Steps:**
1. اعرض backend/driver/GSP/Metal/display/video/power/migration.
2. legacy RTX-unsupported وBochs-only recommendations تقتصر على backend=none.
3. doctor GPU checks للأجهزة التي تستخدم GPU فقط.
4. `--sanitize` يخفي serials/paths/identity.

### T-L3 — accel/optimize/finish guards

**Depends on:** A2.

**Steps:**
1. `accel` يسمح NVMe/RNG المتوافقين.
2. لا balloon افتراضيًا على VFIO.
3. لا virtual display requests على vfio/reims.
4. `optimize/setDisplayMode` يرفضان vfio/reims.
5. `finish` وseed rewriting يستهدفان active stage file.
6. اختبر idempotency.

---

## 18. CI والتكامل العتادي

### T-CI1 — macOS driver workflow

**Depends on:** أول driver source build.  
**Files:** `.github/workflows/driver.yml`.

**Steps:**
1. `make -C driver build test`.
2. unit/golden tests بلا GPU.
3. `make driver-package` artifacts.
4. `ARCHS=x86_64` و`SDKROOT=macosx` مع SDK versions المثبتة.
5. runtime OS compatibility يُثبت على hardware VMs، لا يُستنتج من runner.
6. لا production signing secrets في PRs.

### T-CI2 — hardware workflow

**Depends on:** G1.  
**Files:** `scripts/integration-mxgpu.sh`, manual self-hosted workflow.

**شروط:**
- `MACXVM_MXGPU_INTEGRATION=1`.
- RTX 5060 وIOMMU.
- concurrency محدود بـGPU lease.
- trap يعيد VFIO ويحذف VM.
- لا بيانات مستخدم أو أسرار.

**الحالات:**
- VFIO install/daily/recovery.
- single-gpu rescue matrix.
- Reims shared.
- Metal/Video/Display.
- Power/Migration.
- Sonoma/Tahoe.

### Remote build contract

`macxvm driver build --builder`:
1. أرشف `driver/` وlock file.
2. انقل عبر SSH إلى temporary remote dir.
3. تحقق من archive hash.
4. ابنِ على macOS builder فقط.
5. استرجع الحزمة إلى `artifacts/mxgpu/package/`.
6. تحقق من manifest/signature/hashes.
7. لا kext build commands محليًا على Linux.

---

## 19. الاختبارات وبوابات القبول

### 19.1 Go tests

| الحزمة | الاختبارات |
|---|---|
| config | schema migration، explicit false ReBAR، defaults، invalid combinations |
| argsfile | BuildStage، `#when`، fragments، forbidden incoming، active requirements |
| qemu | display matrix، env merge، VNC recovery، incoming |
| vfio | topologies، duplicate IDs، leases، stale recovery، release transactions |
| opencore | plist round-trip، unique identity، recovery `-mxoff` |
| driver | package signature/archive safety/SBOM/install manifests |
| migration | compatibility، checkpoint parser/fuzz، cancel/rollback |
| store | clone assets/identity/no transient state/sealed checks |
| cmds | registry/help/create/start/driver/migrate |
| accel | لا GPU/balloon/display damage |

### 19.2 Driver tests

| المجال | الاختبارات |
|---|---|
| load probes | minimal IOKit/Metal/Video components على كل OS build |
| ABI | sizes/versions/invalid pointers/client death/cross-client isolation |
| GSP/MXKMS | init/reset/modes/vblank/firmware mismatch |
| Memory | map/unmap/pressure/coherence/DMA isolation |
| Metal | buffers/textures/heaps/blit/compute/render/fences/events |
| Metal advanced | tier2/descriptors/indirect/mesh/ray tracing حسب capability |
| Shaders | AIR versions/atomics/arrays/dynamic samplers/64-bit pointers |
| Surfaces | private/shared/IOSurface/WindowServer/copy/zero-copy |
| Video | decode/encode/XPC/FFmpeg/Safari/QuickTime |
| Display | EDID/hotplug/HiDPI/HDR/1–4 monitors/VRR |
| Power | 50 cycles مع hashes |
| Migration | success/cancel/failure/version mismatch/different BDFs |
| Stress | 24 ساعة mixed workload |

### 19.3 البوابات

| البوابة | موضعها وشرطها |
|---|---|
| G0 | بعد D0: baseline + SBOM/license map |
| G1 | بعد B4: dedicated/single-gpu/Reims + rollback |
| G2 | بعد E4+E2b: MXRM/MXKMS/security |
| G3 | بعد F2: physical framebuffer + IOSurface، دون Metal claim |
| G4 | بعد G4-real ثم T-M2/T-M3/T-M4: Metal الحقيقي والكامل |
| G5 | بعد H3: Video/HDR/multi-monitor |
| G6 | بعد I0: Sleep/Wake |
| G7 | بعد I5: migration backends + rollback |
| G8 | بعد K1: Sonoma/Tahoe/upgrade |
| G9 | بعد J2: sign/package/install/remove/recovery |
| G10 | بعد CI/docs: كل الاختبارات والأدلة والحجم |

---

## 20. الأداء والموارد

### 20.1 Benchmark manifest ثابت

أنشئ `driver/tests/benchmark-manifest.json` يحتوي:
- workload names وsource revisions وhashes.
- build flags وsettings.
- warmup count وmeasured run count.
- Linux/NVK وmacOS versions.
- GPU power state وresolution/refresh.
- raw result لكل run.
- aggregate rule: geometric mean لنسب `MXGPU/LinuxNVK` على نفس مجموعة الأحمال، بلا استبدال workloads.

### 20.2 القياسات

- Metal compute/render suite.
- UI scripted capture بأداة مسماة وثابتة، 1080p60، warmup، capture طويل بما يكفي لـframe-time percentiles؛ لا 20 ثانية فقط لحساب 1% low.
- WindowServer/QEMU CPU/RSS.
- VRAM/GPU utilization.
- Xcode reference project.
- NVMe I/O.
- boot-to-SSH.
- migration downtime.
- sleep/wake resume time.

### 20.3 القبول

| المقياس | الهدف |
|---|---|
| GPU aggregate | ≥90% من Linux/NVK وفق aggregate rule |
| UI | متوسط ≥55 fps، 1% low ≥30 fps |
| Stress | 24 ساعة بلا panic/WindowServer restart |
| Host memory | `MemAvailable ≥4 GiB` بعد VM |
| Xcode | لا تدهور >5% على نفس VM |
| cycles | 20 start/reset و50 sleep/wake |
| migration | downtime موثق واستئناف صحيح |

اختبارات correctness تكشف software fallback أو GPU reinitialization أو prerecorded output.

### 20.4 تقليل الموارد

- لا daemon MacXVM إضافي؛ rescue service خاص بـsingle-gpu فقط.
- Reims native window بدل VNC عند shared.
- لا balloon/hugepages افتراضيًا.
- GPU power management بعد الاستقرار.
- لا خفض RAM بذريعة VRAM.
- لا driver/firmware داخل binary.

---

## 21. مصفوفة التتبع الإلزامية

كل بند يجب أن يبقى مرتبطًا بمهمة واختبار ونتيجة نهائية:

| الميزة | المهمة | الاختبار |
|---|---|---|
| VFIO dedicated | B2 | 20 cycles + physical display |
| single-gpu | B3 | rescue matrix الكاملة |
| Reims shared | B4 | Metal probe + host retains GPU |
| per-VM OpenCore identity | C1 | uniqueness/manifest tests |
| recovery | C2a/C2b/J2 | failed-driver cold boot |
| GSP/GB206 | D1a/E2 | firmware mismatch/init/reset |
| MXKMS | E2b | modes/vblank/suspend/resume |
| IOSurface | F2/F3 | lifetime/coherence/NVK handles |
| Metal basics | T-M1a/T-M1b | mock ABI ثم source-built triangle/vector-add |
| tier2 argument buffers | T-M2 | executable Metal test |
| descriptor arrays/dynamic samplers | T-M2/T-M4 | shader tests |
| indirect command buffers | T-M2 | executable test |
| mesh/ray tracing | T-M2 | GB206 capability + executable tests |
| NVK/NAK | T-M3/T-M3a | direct submission + integrated Metal |
| AIR translation | T-M4/T-M4a | golden + runtime shaders |
| NVDEC/NVENC | H0/H1 | real clients + bitstream/frame checks |
| HDR | H2 | patterns/metadata |
| multi-monitor/VRR | H3 | 1–4 heads/hotplug/refresh matrix |
| Sleep/Wake | I0 | 50 cycles |
| VFIO migration | I3/I4 | capability + migration/rollback |
| Reims migration | I5a/I5 | same-session continuation |
| Sonoma/Tahoe | K0/K1 | ABI/runtime/upgrade rollback |
| package/distribution | J-Release/J0–J3 | signature/clean install/remove/SBOM |
| clone/seal | K2/L0 | seal gates + identity regeneration |
| performance/resources | §20 | benchmark manifest + raw results |

لا يجوز إسقاط بند من القائمة النهائية بسبب capability probe مبهم؛ على العتاد المدعوم يجب تنفيذ الاختبار وإثبات النتيجة.

---

## 22. التوثيق المطلوب

حدّث العربية والإنجليزية معًا واستبدل عبارات NVIDIA/Metal غير الممكن:

- `README.md`, `README_AR.md`
- `docs/GPU.md`, `docs/GPU_AR.md`
- `docs/ACCELERATION.md`, `docs/ACCELERATION_AR.md`
- `docs/ARCHITECTURE.md`, `docs/ARCHITECTURE_AR.md`
- `docs/ARGS_FILE.md`, `docs/ARGS_FILE_AR.md`
- `docs/INSTALLATION.md`, `docs/INSTALLATION_AR.md`
- `docs/TROUBLESHOOTING.md`, `docs/TROUBLESHOOTING_AR.md`
- `docs/BUILDING.md`, `docs/BUILDING_AR.md`
- `docs/SLIM.md`, `docs/SLIM_AR.md`
- `docs/SECURITY.md`
- `docs/DECISIONS.md`
- `CHANGELOG.md`
- `CONTRIBUTING.md`
- `.github/PULL_REQUEST_TEMPLATE.md`

أضف:
- `docs/DRIVER.md`, `docs/DRIVER_AR.md`
- `docs/MIGRATION.md`, `docs/MIGRATION_AR.md`

غطِّ العتاد، أوضاع الملكية، build/install/remove، SBOM، SIP/AMFI، Metal، Video، HDR، power، migration، recovery، والقياسات الفعلية. الميزة قبل بوابتها تُوصف experimental.

---

## 23. حالات الفشل والاستجابة

| الفشل | الاستجابة |
|---|---|
| لا IOMMU | ارفض VFIO؛ لا unsafe no-IOMMU |
| group غير معزولة | ارفض واطبع الأجهزة المتعارضة |
| duplicate PCI IDs | address-based hook |
| lease conflict | ارفض start قبل spawn |
| single-gpu console loss | rescue/rebind/fail-safe؛ امنع الوضع حتى تمر المصفوفة |
| firmware mismatch | fail closed + recovery |
| Metal load failure | راجع D2b signing/discovery؛ لا fake device |
| shader غير مدعوم | error + repro bundle + test |
| WindowServer flicker | `mxnoflip` للتشخيص وإصلاح fence ordering |
| Video encoder missing | راجع D2c/XPC registration |
| migration check stale | أعد check قبل stop |
| destination restore failure | fence الوجهة؛ استأنف المصدر فقط إذا كانت الملكية مؤكدة، وإلا أوقف الجانبين |
| package signature invalid | لا extraction ولا execution |
| interrupted install | recovery وrestore backup المطابق للـmanifest |
| macOS update ABI break | canary/probes/new build |
| OOM | ارفض start أو خفض موارد صراحة |

---

## 24. التراجع

### التعريف

```bash
macxvm start NAME --display recovery
macxvm gpu NAME --remove-driver
macxvm start NAME --display install
```

استخدم backup المطابق لـmacOS/OpenCore/AuxKC manifest.

### VFIO

```bash
macxvm stop NAME --force
macxvm gpu --unbind-vfio --gpu BDF --restore-host
```

استعد udev/initramfs/grub backups وأعد بناء initramfs.

### Migration

1. اتبع حالة `migration/session.json` وepoch، لا تخمّن المالك.
2. إذا لم يُكتب commit record: أوقف وfence destination، ولا تستأنف destination disk.
3. إذا كانت ملكية المصدر مؤكدة، استأنف source إذا كان متوقفًا؛ إذا كانت destination liveness أو ownership غير مؤكدة، أبقِ الجانبين stopped واطلب reconciliation.
4. إذا كتب commit record، فالوجهة هي المالك؛ لا rollback تلقائيًا إلى المصدر.
5. احذف transient state فقط بعد إثبات fencing، وأعد check قبل أي محاولة.

### Sonoma

احذف `sonoma-to-tahoe` واستعد الأصل من backup دون تغيير ختمه.

---

## 25. الترتيب والتبعيات الإلزامية

```text
A0 baseline + Sonoma/Tahoe probe VMs
→ D0 clean-room/SBOM
→ G0
→ CI0 early CI
→ A1 schema/create
→ A2 args/start/readiness
→ B0 discovery
→ B1 host-wide leases
→ C0 plist
→ C1 per-VM OpenCore identity
→ D1 GB206 measurements
→ C2a measured stage configurations
→ B2 dedicated + G1a
→ B3 single-gpu + G1b
→ B4 Reims + G1c
→ G1
→ D1a firmware provisioning/provenance
→ D2a IOKit load probes on both probe VMs
→ D2b Metal load probe on both probe VMs
→ D2c Video load probes on both probe VMs
→ J-Dev development signing/load
→ E0 MXRM skeleton
→ E1 BAR/IRQ/DMA
→ E2 GSP
→ E2b MXKMS
→ C2b runtime boot handoff
→ E3 memory/channels
→ E4 secure user client
→ G2
→ F0 MXFB
→ F1 MXAGDC
→ F2 IOSurface independent of NVK
→ G3
→ T-M0 Metal discovery
→ T-M1a Metal objects with mock backend
→ T-M3a minimal NVK XNU
→ T-M4a minimal AIR
→ T-F3 IOSurface/NVK integration
→ T-M1b first real Metal + G4-real
→ T-M2 Metal features
→ T-M3 NVK completion
→ T-M4 AIR completion
→ G4 final
→ H0 NVDEC
→ H1 NVENC
→ H2 HDR
→ H3 multi-monitor/VRR
→ G5
→ I0 Sleep/Wake
→ G6
→ I1 migration plan
→ I2 checkpoint
→ I3 VFIO migration
→ I4 cooperative migration + G7a
→ I5a Reims save/load
→ I5 Reims migration
→ G7
→ J-Release(Tahoe) + J0(Tahoe) + J1(Tahoe)
→ K0 Sonoma + J-Release/J0/J1(Sonoma)
→ K1 Sonoma→Tahoe + repeat J-Release/J0/J1(Tahoe)
→ G8
→ J2 remove/recovery on both supported OS targets
→ G9
→ J3 distribution/updates
→ K2 finish/seal
→ L0 clone
→ L1 delete/recovery
→ L2 status/doctor
→ L3 accel/optimize guards
→ CI1/CI2
→ docs
→ G10
```

قواعد منع الاستعجال:
- لا Metal hardware test قبل NVK وMXAIR.
- لا F3 قبل F2 وT-M3a.
- لا production package قبل J-Release.
- لا migration قبل Sleep/Wake وstate model.
- لا final acceptance في النهاية إذا كانت أي بوابة وسيطة بلا evidence.

---

## 26. تعريف الإنجاز النهائي

- [ ] كل المهام والتبعيات والبوابات G0–G10 مكتملة بأدلة.
- [ ] MXGPU مستقل ولا يعتمد على NullMoth أو ملفات Apple من المستودع.
- [ ] RTX 5060 وAudio ممرّران فعليًا عبر VFIO.
- [ ] dedicated وsingle-gpu وReims وinstall/daily/recovery تعمل.
- [ ] RTX كرت العرض الرئيسي في daily بلا virtual VGA.
- [ ] MXRM/MXKMS/GSP/channels/memory/fences/reset مستقرة وآمنة بين clients.
- [ ] MXFB/MXAGDC/MXAccel وWindowServer/IOSurface مستقرة.
- [ ] أول Metal حقيقي وكل Metal features المعلنة مجتازة لاختبارات source-built.
- [ ] tier2/descriptors/indirect/mesh/ray tracing مختبرة حيث يدعمها GB206.
- [ ] VideoToolbox encode/decode يعملان من التطبيقات الفعلية.
- [ ] HDR و1–4 monitors وHiDPI/hotplug/VRR حيث يدعمها العتاد مختبرة.
- [ ] Sleep/Wake يحافظ على المحتوى.
- [ ] VFIO وReims Live Migration يعملان مع compatibility/rollback.
- [ ] Sonoma وTahoe مدعومان وترقية Sonoma مُختبرة.
- [ ] package signature/SBOM/install/remove/recovery/distribution ناجحة.
- [ ] clone/seal/delete/status/doctor/finish/accel/optimize لا تكسر GPU.
- [ ] Go CI وmacOS CI وhardware CI تعمل.
- [ ] `go.mod` بلا dependencies جديدة و`bin/macxvm <10 MiB`.
- [ ] الوثائق العربية والإنجليزية وDECISIONS وCHANGELOG محدثة.
- [ ] مصفوفة التتبع في القسم 21 كلها مرتبطة بنتائج executable tests.

---

## 27. مراجع تنفيذية

- NVIDIA open modules: `https://github.com/NVIDIA/open-gpu-kernel-modules`
- Linux VFIO: `https://docs.kernel.org/driver-api/vfio.html`
- QEMU VFIO: `https://www.qemu.org/docs/master/system/devices/vfio.html`
- QEMU VFIO migration: `https://www.qemu.org/docs/master/devel/migration/vfio.html`
- Reims: `https://github.com/steelbrain/reims-vgpu`
- OpenCore: `https://dortania.github.io/OpenCore-Install-Guide/`
- RTX 5060/KVM/VideoToolbox behavior: `https://github.com/nullmoth/nvidia-macos-driver/issues/275`
- Behavioral reference only: `https://github.com/nullmoth/nvidia-macos-driver/blob/main/docs/HOW-IT-WORKS.md`

**قاعدة أخيرة:** لا تعتبر وجود API أو document أو kext محمّل دليلًا على الميزة. الدليل هو executable test على العتاد الفعلي مع rollback وraw evidence.
