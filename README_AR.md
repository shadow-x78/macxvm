<a id="credits"></a>
# الشكر

تقف MacXVM على أكتاف من جعلوا تشغيل macOS على QEMU ممكنًا أصلًا. لا توزّع أي برنامج من Apple، ولا برامج ثابتة، ولا محمّل إقلاع، ولا مفاتيح، ولا تُشغّل إلا ما تقدّمه أنت.

- **[OSX-KVM](https://github.com/kholia/OSX-KVM)** لـ *kholia* — الدليل وسكربت الإقلاع الأصليين لتشغيل macOS على QEMU/KVM، والمرجع الذي يتكيّف معه ملف الوسائط.
- **[ultimate-macOS-KVM](https://github.com/Coopydood/ultimate-macOS-KVM)** لـ *Coopydood* — الدليل المُصان الذي يحوّل سكربت الإقلاع إلى تثبيت كامل قابل للتكرار.
- **[QEMU](https://www.qemu.org)** — المحاكي الذي يشغّل الضيف.
- **[OpenCore](https://github.com/acidanthera/OpenCorePkg)** — محمّل الإقلاع الذي تستخدمه الأدلّة، يثبّته ويضبطه أنت.
- **[Apple](https://www.apple.com)** — macOS وXcode وiOS، علامات تجارية لشركة Apple Inc.، لا يوزّعها هذا المشروع ولا يقرّبه.

MacXVM مشروع مفتوح المصدر مستقل، غير تابع لشركة Apple Inc. وغير معتمد منها.

---

<div align="center">

<a href="https://github.com/shadow-x78/macxvm">
  <img src="https://raw.githubusercontent.com/shadow-x78/macxvm/main/assets/logo/macxvm-banner.png" alt="MacXVM banner" width="100%" />
</a>

# MacXVM

شغّل ضيوف macOS خفيفين مع Xcode على مضيف Linux.

[![الإصدار](https://img.shields.io/badge/version-0.1.0-007AFF?style=flat-square&logo=semver)](CHANGELOG.md)
[![الترخيص](https://img.shields.io/badge/license-MIT-1D1D1F?style=flat-square)](LICENSE)
![Go](https://img.shields.io/badge/go-1.22%2B-00ADD8?style=flat-square&logo=go)
![المنصة](https://img.shields.io/badge/platform-Linux%20x86__64-5E5CE6?style=flat-square&logo=linux)

</div>

---

## اللغة

<a href="README.md">🇬🇧 English</a> · <a href="README_AR.md">🇸🇦 العربية</a>

---

## فهرس المحتويات

- [نظرة عامة](#overview)
- [الإطار القانوني والحدود](#legal)
- [المتطلبات](#requirements)
- [المزايا](#features)
- [ملف الوسائط](#args-file)
- [البدء السريع](#quick-start)
- [الملفات التعريفية](#profiles)
- [التخفيف](#slimming)
- [بناء المشاريع](#building)
- [مرجع الأوامر](#commands)
- [السرعة](#speed)
- [تنظيم المشروع](#project-structure)
- [التوثيق](#documentation)
- [المساهمة](#contributing)
- [الترخيص](#license)

---

<a id="overview"></a>
## نظرة عامة

تحوّل MacXVM جهاز Linux إلى مزرعة بناء لمشاريع macOS. تسجّل ضيوف macOS، وتشغّلهم تحت QEMU/KVM، وتخفّفهم حتى يهدأوا في وضع الخمول، وتثبّت Xcode داخلهم، وتبني مشاريعك من Linux عبر `rsync` و`ssh`.

الأداة ملف تنفيذي واحد ثابت بحجم نحو 4 ميغابايت بلا خادم دائم. العملية الوحيدة التي تبقى حيّة هي QEMU نفسها، فلا يحمل المضيف الخامل شيئًا إطلاقًا.

```
macxvm create demo --args-file ./args.txt --assets ./assets
macxvm start demo --install --vnc
macxvm slim demo --level 1
macxvm tools install demo clt
macxvm xcode install demo --xip ./Xcode.xip
macxvm base seal demo
macxvm build --project . --scheme App
```

صورة أساس ختم مع طبقات qcow2 هي ما يجعل هذا سريعًا: الجهاز الجديد يكلّف بضعة مئات كيلوبايت وثانية واحدة، لا نسخة من مئة غيغابايت. وتختفي الطبقات مع `macxvm run`، الذي يشغّل جهازًا مؤقتًا وينفّذ أمرًا واحدًا فيه ثم يحذفه.

لا تصف MacXVM جهاز macOS بنفسها أبدًا. تعريف الجهاز يعيش في ملف وسائط تكتبه أنت، ف تستبدل MacXVM فيه عددًا قليلًا من العناصر النائبة وتشغّل QEMU بالنتيجة.

<a id="legal"></a>
## الإطار القانوني والحدود

> MacXVM مشروع مفتوح المصدر مستقل، غير تابع لشركة Apple Inc. وغير معتمد منها. macOS وXcode وiOS علامات تجارية لشركة Apple Inc. لا يوزّع MacXVM أي برنامج من Apple؛ المستخدم يقدّم كل برمجيات Apple ويتحمّل مسؤوليتها والامتثال لشروط ترخيصها.

اقرأ هذا قبل أن تثبّت شيئًا:

1. **ترخيص macOS من Apple يسمح بتشغيل macOS داخل جهاز افتراضي على أجهزة من إنتاج Apple فقط.** تشغيل macOS تحت QEMU/KVM على حاسوب مكتبي خرق لترخيص Apple، وقد نفّذته Apple أمام القضاء. MacXVM مُشغِّل فقط: لا يوزّع أي برنامج من Apple ولا برامج ثابتة ولا مفاتيح ولا محمّلات إقلاع، ولا يشغّل إلا ما تقدّمه. **المسؤولية عليك.** والبدائل المطابقة هي جهاز Mac (استخدم مواصفة أجهزة Apple) أو Mac مستأجر في السحابة.
2. **x86_64 فقط.** الضيف نظام macOS بمعمارية x86_64. تدعم Apple أجهزة Intel حتى macOS 26، فيقصر الضيف على macOS ≤ 26 وبالتالي على إصدارات Xcode التي تعمل عليه. اختر إصدار macOS وإصدار Xcode معًا بالرجوع إلى ملاحظات إصدار Xcode. لا يثبّت هذا المشروع أزواج إصدارات بشكل ثابت.
3. **المضيف.** Linux بمعمارية x86_64 مع المحاكاة الافتراضية (Intel VT-x أو AMD-V)، و`/dev/kvm` يستطيع مستخدمك استعماله، وQEMU بإصدار 6.0 أو أحدث. لا دعم لمضيف Linux بمعمارية ARM.
4. **لا تسريع رسومي.** واجهة المستخدم ومحاكي iOS يُعرضان برمجيًا. البناء سليم، والمحاكي بطيء.
5. **الهشاشة.** قد يكسر تحديث نظام macOS داخل جهاز افتراضي كهذا الإقلاع. المستوى الأول من التخفيف يطفئ التحديثات التلقائية، ولا ينبغي أن تشغّل تحديثات النظام داخل الضيف.
6. **أصول الإقلاع مسؤوليتك.** تعريف الجهاز ومحمّل الإقلاع والبرامج الثابتة وصورة مثبّت macOS تأتي منك، مثلًا باتباع توثيق المشاريع المفتوحة المذكورة في [الشكر](#credits). لا ينسخ MacXVM أي محتوى منها، ولا ينزّل أي مثبّت من Apple، ولا يؤتمت تثبيت macOS.

<a id="requirements"></a>
## المتطلبات

| المتطلب | التفصيل |
|----------|---------|
| المضيف | Linux x86_64، محاكاة افتراضية عتادية، و`/dev/kvm` يستطيع مستخدمك استعماله |
| QEMU | `qemu-system-x86_64` و`qemu-img`، بإصدار 6.0 أو أحدث |
| أدوات المضيف | `ssh` و`scp` و`rsync` و`ssh-keygen` |
| الضيف | macOS بمعمارية x86_64 ≤ 26، تثبّته أنت، مع تفعيل Remote Login |
| Go | 1.22 أو أحدث، لبناء MacXVM فقط |

افحص كل ذلك بـ:

```bash
macxvm doctor
```

<a id="features"></a>
## المزايا

- تسجيل ضيوف macOS وإقلاعهم وإيقافهم ونسخهم وحذفهم تحت QEMU/KVM
- ملف وسائط من المستخدم كتعريف للجهاز: بلا طراز معالج ثابت، ولا مفاتيح SMC، ولا قيم SMBIOS، ولا إعدادات OpenCore، ولا مسارات برامج ثابتة
- صور أساس ختم مع طبقات qcow2: الجهاز الجديد يكلّف بضعة مئات كيلوبايت وثانية واحدة
- أجهزة مؤقتة عبر `macxvm run`: تُحذف الطبقة بعدها، حتى عند مقاطعة الأمر
- مستويان من التخفيف يخفّضان كلفة الخمول في الضيف، مع قائمة منع تحمي SSH والشبكة، واستعادة تعكس التغيير
- تثبيت أدوات سطر الأوامر وHomebrew وXcode داخل الضيف من المضيف
- `xcodebuild` و`swift build` على ضيف بعيد، مع الاحتفاظ بالبيانات المشتقة بين البناءات
- ملف مشروع يحمل قيم البناء والجهاز وصورة الأساس الافتراضية
- وضع بلا واجهة افتراضيًا، وشاشة VNC على واجهة loopback حين تحتاج رؤية الشاشة
- ملف تنفيذي واحد ثابت بحجم نحو 4 ميغابايت، بمكتبة Go القياسية فقط، بلا وحدات خارجية، بلا خادم دائم، بلا عملية خلفية

<a id="args-file"></a>
## ملف الوسائط

ملف الوسائط هو تعريف الجهاز الذي يأتي بك. يستبدل MacXVM فيه عددًا قليلًا من العناصر النائبة ويضيف الأعلام التي يحتاجها لإدارة الجهاز.

```bash
qemu-system-x86_64 \
  -cpu host \
  -smp {{CPUS}} \
  -m {{RAM_MIB}} \
  -drive file={{DISK_IMG}},format=qcow2,{{DISK_OPTS}} \
  -drive file={{ASSETS}}/efi.img,format=raw \
  -netdev {{NETDEV}} \
  -device virtio-net-pci,mac={{MAC}}

#install -drive file={{ASSETS}}/installer.img,format=raw
```

| العنصر | القيمة |
|--------|--------|
| `{{CPUS}}` | عدد أنوية المعالج في الجهاز |
| `{{CORES}}` | نصف الأنوية، ولا يقل عن واحد |
| `{{RAM_MIB}}` | ذاكرة الجهاز بالميغابايت |
| `{{DISK_IMG}}` | المسار المطلق لقرص الجهاز |
| `{{DISK_OPTS}}` | `cache=writeback`، أو `cache=unsafe` لجهاز مؤقت |
| `{{NVRAM}}` | المسار المطلق لملف NVRAM، فقط حين يملك الجهاز واحدًا |
| `{{ASSETS}}` | مجلد أصول الجهاز |
| `{{MAC}}` | عنوان MAC للجهاز |
| `{{NETDEV}}` | `user,id=net0,hostfwd=tcp:127.0.0.1:<port>-:22` |

العناصر `{{DISK_IMG}}` و`{{NETDEV}}` و`{{MAC}}` و`{{RAM_MIB}}` و`{{CPUS}}` مطلوبة. ويضيف MacXVM بنفسه `-name` و`-pidfile` و`-qmp` و`-display` و`-vnc`، فلا يجوز أن يحتويها ملف الوسائط — وأغلب سكربتات الإقلاع في المشاريع الأخرى تستخدم `-monitor stdio`، فاحذف ذلك السطر.

الدليل الكامل، بما فيه تكييف سكربت إقلاع جاهز، في [docs/ARGS_FILE_AR.md](docs/ARGS_FILE_AR.md).

<a id="quick-start"></a>
## البدء السريع

```bash
# 1. سجّل الجهاز
macxvm create demo --args-file ./args.txt --assets ./assets

# 2. ثبّت macOS داخل الضيف
macxvm start demo --install --vnc
#    صل بعميل VNC بالعنوان 127.0.0.1:<المنفذ الذي يطبعه macxvm> واتّبع دليل
#    التثبيت الخاص بالمشروع الذي جاءت منه أصولك
```

<a id="first-boot-checklist"></a>
### قائمة إقلاع أول مرة

1. أنشئ المستخدم `admin` بكلمة مرور، وتخطَّ Apple ID وiCloud وSiri والتحليلات.
2. إعدادات النظام، ثم عام، ثم المشاركة، ثم **Remote Login**: مفعّل.
3. في طرفية داخل الضيف:

```bash
sudo sh -c 'echo "admin ALL=(ALL) NOPASSWD: ALL" > /etc/sudoers.d/admin'
```

4. على المضيف، سلّم مفتاح MacXVM إلى الضيف:

```bash
macxvm ssh-key --install demo
```

5. خفّف الضيف وثبّت الأدوات:

```bash
macxvm slim demo --level 1
macxvm tools install demo clt
macxvm xcode install demo --xip ./Xcode.xip
macxvm slim demo --level 2
```

6. احتفظ بالنتيجة كصورة أساس:

```bash
macxvm stop demo
macxvm base seal demo
```

7. ابنِ:

```bash
macxvm clone demo work
macxvm build --vm work --project . --scheme App
```

<a id="profiles"></a>
## الملفات التعريفية

| الملف | المحتوى | الموارد الافتراضية | الاستخدام |
|-------|---------|---------------------|-----------|
| `xcode` (افتراضي) | macOS خفيف + أدوات سطر الأوامر + Xcode | 4 أنوية / 8192 MiB / 100 GiB | `xcodebuild` والمحاكيات وملفات `.xcodeproj` و`.xcworkspace` |
| `cli` | macOS خفيف + أدوات سطر الأوامر | 2 أنوية / 4096 MiB / 32 GiB | حزم Swift لمنصة macOS وHomebrew والسكربتات |

| | `cli` | `xcode` |
|---|---|---|
| `swift build` / `swift test` لمنصة macOS | نعم | نعم |
| حزم Homebrew وgit والسكربتات | نعم | نعم |
| `xcodebuild` و`.xcodeproj` و`.xcworkspace` | لا (رمز الخروج 9) | نعم |
| حزمة iOS SDK ومحاكي iOS | لا (أدوات سطر الأوامر فيها حزمة macOS فقط) | نعم، بعرض برمجي |

<a id="slimming"></a>
## التخفيف

```bash
macxvm slim demo --level 1
macxvm slim demo --level 2 --measure
macxvm slim demo --restore
```

المستوى الأول يطفئ فهرسة Spotlight وTime Machine والتحديثات التلقائية والحركات ومؤقتات النوم. والمستوى الثاني يعطّل أيضًا مهام launchd المذكورة في `resources/slim-labels.txt`، ويتخطّى كل مهمة غير موجودة في إصدار macOS لديك.

لا يُمَس إلا التسميات الموجودة في ذلك الملف. وقائمة منع في الشيفرة تحذف كل تسمية تكسر SSH أو الشبكة أو خادم النوافذ. لا يُعدَّل مجلد النظام أبدًا، ولا تُغيَّر SIP، ولا يُحذف أي تطبيق من النظام.

التفاصيل في [docs/SLIM_AR.md](docs/SLIM_AR.md).

<a id="building"></a>
## بناء المشاريع

```bash
macxvm build --project . --scheme App
```

يُنسخ المشروع إلى الضيف بـ `rsync -a`، ويجري البناء هناك بـ `xcodebuild` أو `swift build`، وتُبثّ المخرجات. ولا يُشغَّل الجهاز إلا إن لم يكن يعمل، ويُوقف من جديد بعدها حتى عند فشل البناء.

```bash
macxvm build --ephemeral --from base-xcode --project . --scheme App
```

يبني في طبقة مؤقتة تُحذف بعدها. الدليل الكامل، بما فيه ملف المشروع وجلب النتائج، في [docs/BUILDING_AR.md](docs/BUILDING_AR.md).

<a id="commands"></a>
## مرجع الأوامر

| الأمر | الغرض |
|-------|-------|
| `macxvm create NAME --args-file FILE [...]` | تسجيل جهاز افتراضي |
| `macxvm start NAME [--install] [--vnc] [--force]` | تشغيل QEMU |
| `macxvm stop NAME [--force] [--timeout SECONDS]` | إيقاف الضيف بلطف إن أمكن |
| `macxvm list [--json]` / `macxvm status NAME [--json]` | عرض الأجهزة |
| `macxvm ssh NAME [-- ARGS...]` | جلسة SSH تفاعلية |
| `macxvm exec NAME -- CMD...` | تنفيذ أمر، مع تمرير رمز الخروج |
| `macxvm cp SRC DST` | نسخ ملفات، ومسارات الضيف بصيغة `NAME:/path` |
| `macxvm ssh-key [--install NAME]` | عرض مفتاح SSH أو تثبيته |
| `macxvm clone SRC DST [--full]` | نسخ جهاز، بطبقة فوق أساس ختم افتراضيًا |
| `macxvm delete NAME [--yes]` | حذف جهاز |
| `macxvm slim NAME [--level 1\|2] [--measure] [--restore]` | تخفيف الضيف |
| `macxvm tools install NAME clt\|brew` | تثبيت أدوات سطر الأوامر أو Homebrew |
| `macxvm tools brew NAME PKG...` | تثبيت حزم Homebrew |
| `macxvm tools list NAME` | عرض الأدوات المثبّتة |
| `macxvm xcode install NAME --xip PATH [--platform iOS]` | تثبيت Xcode |
| `macxvm xcode clean NAME` | حذف مخابئ Xcode |
| `macxvm base seal NAME` / `unseal NAME` / `list` | إدارة صور الأساس |
| `macxvm run --from BASE [...] -- CMD...` | تنفيذ أمر في جهاز مؤقت |
| `macxvm prune [--all]` | حذف بقايا الأجهزة المؤقتة |
| `macxvm build [...]` | بناء مشروع داخل ضيف |
| `macxvm compact NAME` | تصغير قرص جهاز متوقف |
| `macxvm init [--profile P]` | كتابة ملف `macxvm.json` |
| `macxvm up` / `macxvm down [--delete]` | تشغيل وإيقاف جهاز المشروع |
| `macxvm doctor` | فحص المضيف |
| `macxvm version` | طباعة الإصدار |
| `macxvm completion bash\|zsh` | طباعة سكربت إكمال الصدفة |

رموز الخروج: `0` نجاح، `1` خطأ، `2` استخدام خاطئ، `3` غير موجود، `4` يعمل بالفعل، `5` لا يعمل، `6` مضيف غير مدعوم، `7` انتهاء المهلة، `8` ملف وسائط غير صالح، `9` عدم تطابق الملف التعريفي أو الأدوات.

<a id="speed"></a>
## السرعة

| العملية | الهدف | المقاس على المضيف المرجعي |
|---------|-------|----------------------------|
| `list` و`status` | أقل من 100 مللي ثانية | 3 مللي ثانية |
| الملف التنفيذي | أقل من 6 ميغابايت | 4.07 ميغابايت، ثابت، `linux/amd64` |
| نسخ طبقة فوق أساس ختم | أقل من ثانية | أقل من ثانية، وبضعة مئات كيلوبايت على القرص |
| الكلفة الخاملة على المضيف | لا شيء | لا تبقى أي عملية من MacXVM |

قِيست هذه الأرقام على الجهاز الذي يبني هذا المشروع، وفيه أربعة أجهزة مسجّلة.depends على تعريف جهازك في ملف الوسائط وعلى قرص الضيف. ويقيس `scripts/bench.sh BASE` النسخ والإقلاع والتشغيل المؤقت على مضيفك.

<a id="project-structure"></a>
## تنظيم المشروع

```
macxvm/
├── cmd/macxvm/                    # نقطة الدخول للملف التنفيذي
├── internal/                      # حزم الأداة
│   ├── exitcode/                  # نوع الخطأ الذي يحمل رمز الخروج
│   ├── paths/                     # مجلد الحالة ومجلد التشغيل
│   ├── names/                     # قاعدة اسم الجهاز
│   ├── sizeparse/                 # تحليل قيم --ram
│   ├── config/                    # وصف الجهاز المحفوظ
│   ├── store/                     # التسجيل والنسخ والختم والحذف
│   ├── argsfile/                  # محلّل ملف الوسائط وتوسيعه
│   ├── runstate/                  # كشف العملية وقفل العمليات
│   ├── qmp/                       # عميل QMP
│   ├── qemu/                      # التشغيل والجاهزية وبناء وسائط QEMU
│   ├── sshx/                      # بُناة وسائط SSH وSCP وrsync
│   ├── slim/                      # سياسة التخفيف ومنفّذه
│   ├── tools/                     # أدوات سطر الأوامر وHomebrew
│   ├── xcode/                     # تثبيت Xcode
│   ├── project/                   # كشف المشروع وملف macxvm.json
│   ├── doctor/                    # فحوص المضيف
│   ├── hostcheck/                 # حقائق المضيف وواجهة Runner
│   └── cmds/                      # ملف واحد لكل أمر
├── resources/                     # سكربتات الضيف وقائمة التسميات
├── scripts/                       # التثبيت والقياس وفحص النمط
├── docs/                          # أدلة ثنائية اللغة (إنجليزي + عربي)
├── assets/logo/                   # الشعار واللافتة ومعاينة التواصل
├── .github/                       # مسارات العمل وقوالب Issues وPull Requests
├── Makefile                       # build وtest وinstall وclean
├── CHANGELOG.md                   # ملاحظات الإصدارات
├── CONTRIBUTING.md                # كيف تساهم
├── CODE_OF_CONDUCT.md             # قواعد المجتمع
├── SECURITY.md                    # كيف تبلّغ عن ثغرة
└── README.md, README_AR.md        # نقطتا الدخول
```

<a id="documentation"></a>
## التوثيق

| المستند | الوصف |
|---------|-------|
| [ARCHITECTURE_AR.md](docs/ARCHITECTURE_AR.md) | الحزم ودورة الحياة وتخزين الحالة وإدارة العملية |
| [ARGS_FILE_AR.md](docs/ARGS_FILE_AR.md) | تعريف الجهاز: الصيغة والعناصر النائبة والأعلام الممنوعة والسكربتات الجاهزة |
| [SLIM_AR.md](docs/SLIM_AR.md) | مستويات التخفيف وقائمة المنع والاستعادة والقياس |
| [BUILDING_AR.md](docs/BUILDING_AR.md) | سير عمل البناء وملف المشروع والمزامنة والبناء المؤقت |
| [TROUBLESHOOTING_AR.md](docs/TROUBLESHOOTING_AR.md) | مشكلات المضيف والجهاز والضيف والبناء الشائعة |
| [DECISIONS.md](docs/DECISIONS.md) | كل انحراف عن المواصفة وكل غموض حُلّ |

كل مستند موجود بالإنجليزية والعربية.

<a id="contributing"></a>
## المساهمة

1. انسخ المستودع وأنشئ فرعًا (`feature/` أو `fix/` أو `docs/` أو `chore/`).
2. أبقِ `go vet ./...` و`go test ./...` و`make build` خضراء، وأبقِ ترويسات الملفات والتنسيق كما هما.
3. أضف مدخلًا في `CHANGELOG.md`.
4. اكتب رسالة الالتزام `macxvm | <type>: <description>` وافتح طلب سحب ضد `main`.

اقرأ [دليل المساهمة](CONTRIBUTING.md) و[ميثاق السلوك](CODE_OF_CONDUCT.md). بلّغ عن المشكلات عبر [Issues على GitHub](https://github.com/shadow-x78/macxvm/issues).

<a id="license"></a>
## الترخيص

موزّع بموجب [رخصة MIT](LICENSE).

---

<div align="center">

من إعداد <a href="https://github.com/shadow-x78">shadow-x78</a> ·
[سجل التغييرات](CHANGELOG.md) ·
[الأمان](SECURITY.md)

<sub>&copy; 2026 MacXVM</sub>

</div>
