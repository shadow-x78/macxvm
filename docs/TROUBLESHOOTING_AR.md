<div align="center">

# حل المشكلات - MacXVM

[![الإصدار](https://img.shields.io/badge/version-0.5.3-007AFF?style=flat-square&logo=semver)](../CHANGELOG.md)
[![الترخيص](https://img.shields.io/badge/license-MIT-1D1D1F?style=flat-square)](../LICENSE)
![Go](https://img.shields.io/badge/go-1.22%2B-00ADD8?style=flat-square&logo=go)
![المنصة](https://img.shields.io/badge/platform-Linux%20x86__64-5E5CE6?style=flat-square&logo=linux)

</div>

---

## اللغة

<a href="TROUBLESHOOTING.md">🇬🇧 English</a> · <a href="TROUBLESHOOTING_AR.md">🇸🇦 العربية</a>

---

## فهرس المحتويات

- [ابدأ من هنا](#start)
- [المضيف](#host)
- [الجهاز](#machine)
- [الضيف](#guest)
- [البناء](#build)
- [أوامر التشخيص](#diagnostics)

---

<a id="start"></a>
## ابدأ من هنا

```bash
macxvm doctor
```

يفحص التقرير نظام التشغيل و`/dev/kvm` وعلامات المحاكاة الافتراضية في المعالج وQEMU ووحدة KVM وأدوات المضيف ومساحة القرص الحرة وذاكرة المضيف والأجهزة المسجلة. وينتهي بالتنببيه القانوني.

<a id="host"></a>
## المضيف

**`cannot use /dev/kvm (add your user to the 'kvm' group and log in again)`**

لا يستطيع مستخدمك استعمال جهاز KVM. أضف نفسك إلى المجموعة وسجل الدخول من جديد:

```bash
sudo usermod -aG kvm $USER
```

اخرج من الجلسة وادخل فيها، أو افتح جلسة جديدة. صدفة جديدة لا تكفي، فالمجموعة تقرأ عند بدء الجلسة.

**`MacXVM needs an x86_64 Linux host with KVM and QEMU (run: macxvm doctor)`**

إما أن المضيف ليس Linux بمعمارية x86_64، أو أن QEMU غير مثبت. على Fedora:

```bash
sudo dnf install qemu-system-x86_64 qemu-img
```

**ينبه `doctor` على `ignore_msrs`.**

عدة أدلة لتشغيل macOS على KVM تحتاج أن تتجاهل وحدة KVM سجلات المعالج الفريدة. راجع README المشروع الذي جاءت منه أصول الإقلاع. لا يغير MacXVM هذا الإعداد نيابة عنك.

**ينبه `doctor` أن QEMU لا يختار KVM تلقائيا.**

بعض بناءات QEMU لا تستعمل KVM إلا حين يسمى المسرع صراحة، والجهاز الذي يطلب معالج المضيف يحتاجه. أضف `-accel kvm` إلى ملف وسائط الجهاز:

```bash
qemu-system-x86_64 \
  -accel kvm \
  -cpu host \
  ...
```

**`run directory path too long; set MACXVM_RUN to a shorter path`**

يجب أن يناسب مسار مقبس QMP حجم `sun_path`. وجه `MACXVM_RUN` إلى مجلد أقصر:

```bash
export MACXVM_RUN=/tmp/macxvm
```

<a id="machine"></a>
## الجهاز

**`VM 'X' is already running`**

الجهاز يعمل. يعرض `macxvm status X` حالته، ويوقفه `macxvm stop X`.

**`'X' is a sealed base image; clone it first (macxvm clone X NEW)`**

الأساس الختم ملف backing للطبقات ولا يقلع أبدا. انسخه واقلع النسخة.

**`only sealed base images can be overlay-cloned; use --full or seal it first`**

ختم الأساس أولا ب `macxvm base seal X`، أو انسخه بالخيار `--full` الذي ينسخ القرص كله.

**`'X' is the base of other VMs (a, b); delete them first`**

الأساس ما زال مستخدما. احذف الطبقات أولا، أو ألغ الختم ب `macxvm base unseal X`.

**`another macxvm operation is running on 'X'`**

أمر آخر من MacXVM يحمل قفل الجهاز. انتظر حتى ينتهي.

**`VM 'X' is not running`**

الجهاز متوقف. `macxvm start X` يشغله. وإن كانت عملية QEMU ما زالت حية فقدت ملف المعرف، فتجدها الأداة عبر سطر أوامرها وتوقفها على أي حال.

**الجهاز يبدأ ويتوقف فورا.**
اقرأ سجل الجهاز:

```bash
tail -n 40 ~/.local/share/macxvm/vms/X/console.log
```

السبب الأكثر شيوعا مسار خاطئ في ملف الوسائط، أو ملف ناقص في مجلد الأصول.

**`QEMU exited before it was ready (see ...)`**

تطبع آخر أسطر سجل الطرفية فوق الرسالة، وهي تذكر غالبا الوسيط الذي لم يقبله QEMU.

<a id="guest"></a>
## الضيف

**`could not reach 'X' over SSH after 240 s (is Remote Login enabled? see README: First boot checklist)`**

داخل الضيف افتح إعدادات النظام، ثم عام، ثم المشاركة، وفعل **Remote Login**. يجب أن يملك مستخدم الجهاز كلمة مرور، وصلاحية sudo بلا كلمة مرور التي يطلبها [قائمة إقلاع أول مرة](../README_AR.md#قائمة-إقلاع-أول-مرة) تتيح ل MacXVM تشغيل الأوامر التي تحتاج root.

**`Command Line Tools are not installed in 'X' (run: macxvm tools install X clt)`**

ثبتها أولا:

```bash
macxvm tools install X clt
```

**`this profile has no Xcode; use profile 'xcode'`**

جهاز الملف التعريفي `cli` لا يحتوي Xcode. أنشئ جهازا من الملف `xcode` لبناء مشروع Xcode.

**الضيف بطيء بعد تثبيت Xcode.**
أول تشغيل ل Xcode وأول بناء يملآن المخابئ. يستغرق ذلك وقتا مرة واحدة، على قرص الضيف.

**الضيف لا يرد بعد التخفيف.**
استعده وأعد تشغيله:

```bash
macxvm slim X --restore
```

<a id="build"></a>
## البناء

**`unsupported project`**

مجلد المشروع لا يحتوي `.xcworkspace` ولا `.xcodeproj` ولا `Package.swift` في مستواه الأعلى. الكشف ينظر إلى المستوى الأعلى فقط.

**`the project has several schemes (App, App-Staging), use --scheme`**

سئل الضيف عن سكيماته فوجد أكثر من واحد. اختر واحدا بالخيار `--scheme`.

**البناء لا يجد المشروع داخل الضيف.**
مسار المشروع جزء من اسم مجلد العمل. فمساران مختلفان للمشروع نفسه يستعملان مجلدين مختلفين، فلا تشارك المخابئ. استخدم المسار نفسه دائما، أو استخدم رابطا رمزيا.

**`the local path '~/.bashrc' must be relative to the project`**
مسارات `fetch` يجب أن تبقى داخل المشروع. راجع [BUILDING_AR.md](BUILDING_AR.md#جلب-النتائج).

**يتوقف البناء عند خطوة التوقيع.**
يحتاج توقيع الشيفرة سلسلة مفاتيح غير مقفلة. استخدم وجهة محاكي أو `CODE_SIGNING_ALLOWED=NO`.

<a id="diagnostics"></a>
## أوامر التشخيص

```bash
macxvm doctor                 # حالة المضيف
macxvm list                    # كل الأجهزة المسجلة
macxvm status X                # حالة جهاز واحد
macxvm list --json             # الشيء نفسه، لبرمجية
tail -n 40 ~/.local/share/macxvm/vms/X/console.log   # مخرجات QEMU
cat ~/.local/share/macxvm/vms/X/config.json           # وصف الجهاز
```

داخل الضيف:

```bash
xcodebuild -version            # هل Xcode موجودة؟
xcode-select -p                # أي مجلد مطورين نشط؟
launchctl print system/com.apple.sshd   # هل SSH حي؟
df -h                          # كم بقي من المساحة؟
```
