<div align="center">

# ملف الوسائط - MacXVM

[![الإصدار](https://img.shields.io/badge/version-0.8.0-007AFF?style=flat-square&logo=semver)](../CHANGELOG.md)
[![الترخيص](https://img.shields.io/badge/license-MIT-1D1D1F?style=flat-square)](../LICENSE)
![Go](https://img.shields.io/badge/go-1.22%2B-00ADD8?style=flat-square&logo=go)
![المنصة](https://img.shields.io/badge/platform-Linux%20x86__64-5E5CE6?style=flat-square&logo=linux)

</div>

---

## اللغة

<a href="ARGS_FILE.md">🇬🇧 English</a> · <a href="ARGS_FILE_AR.md">🇸🇦 العربية</a>

---

## فهرس المحتويات

- [ما هو ملف الوسائط](#what-it-is)
- [الصيغة](#format)
- [العناصر النائبة](#placeholders)
- [العناصر النائبة المطلوبة](#required-placeholders)
- [الأعلام الممنوعة](#forbidden-flags)
- [الأعلام المضافة](#injected-flags)
- [تكييف سكربت إقلاع جاهز](#adapting)
- [التحقق والأخطاء](#validation)
- [مثال عملي](#example)

---

<a id="what-it-is"></a>
## ما هو ملف الوسائط

ملف الوسائط هو تعريف الجهاز. تكتب فيه وسائط QEMU الخاصة بضيفك، فيستبدل MacXVM عددا قليلا من العناصر النائبة، ويضيف الأعلام التي يحتاجها لإدارة الجهاز، ثم يشغل QEMU بالنتيجة.

‏MacXVM لا يعرف شيئا عن macOS على QEMU. لا طراز معالج، ولا مفتاح SMC، ولا قيمة SMBIOS، ولا إعداد OpenCore، ولا مسار برنامج ثابت في أي مكان من الشيفرة. إن كان ضيفك يحتاج أمرا ما فاكتبه في ملف الوسائط.

تنسخ الملف إلى مجلد حالة الجهاز عند تسجيله، وتستخدم النسخة منه بعد ذلك.

<a id="format"></a>
## الصيغة

- وسائط QEMU واحد أو أكثر في كل سطر، مع اقتباس شبيه بالصدفة: `'...'` نص حرفي، و`"..."` تسمح ب `\"` و`\\`، والشرطة المائلة العكسية خارج الاقتباس تهرب الحرف التالي.
- الأسطر الفارغة تتجاهل.
- السطر الذي يبدأ ب `#` تعليق ويتجاهل.
- السطر الذي يبدأ ب `#install ` (مسافة واحدة بالضبط) يستخدم **فقط** عند تشغيل الجهاز بخيار `--install`. استخدمه لوسيط التثبيت وخطوات الإقلاع.
- الشرطة المائلة العكسية في آخر السطر لا تكمل السطر. كل سطر مستقل، وتحذف الشرطة نفسها، فيمكن نسخ سكربت الإقلاع لمشروع آخر كما هو.
- قد يكون الرمز الأول من السطر الأول اسم ملف QEMU، فيحذف لأن MacXVM يشغل QEMU بنفسه.

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
## العناصر النائبة

| العنصر | القيمة |
|--------|--------|
| `{{CPUS}}` | عدد أنوية المعالج في الجهاز |
| `{{CORES}}` | نصف الأنوية، ولا يقل عن واحد |
| `{{RAM_MIB}}` | ذاكرة الجهاز بالميغابايت |
| `{{DISK_IMG}}` | المسار المطلق لقرص الجهاز |
| `{{DISK_OPTS}}` | `cache=writeback`، أو `cache=unsafe` لجهاز مؤقت |
| `{{NVRAM}}` | المسار المطلق لملف NVRAM، فقط حين يملك الجهاز واحدا |
| `{{ASSETS}}` | مجلد أصول الجهاز |
| `{{MAC}}` | عنوان MAC للجهاز |
| `{{NETDEV}}` | `user,id=net0,hostfwd=tcp:127.0.0.1:<port>-:22` |

يستبدل العنصر داخل أي رمز، فيعمل `-smp {{CPUS}}` و`-smp={{CPUS}}` معا. العنصر المجهول خطأ صريح، ولا يتحول أبدا إلى نص فارغ.

`{{NVRAM}}` و`{{ASSETS}}` يفشلان إن لم يكن للجهاز ملف NVRAM أو مجلد أصول. أنشئ الجهاز بالخيارين `--nvram-template` و`--assets` إن احتجتهما.

شبكة الضيف تحتاج سطرين معا. سطر `-netdev` يبني شبكة QEMU باسم `net0`، وسطر `-device` يجب أن يشير إلى ذلك الاسم ب `netdev=net0`. بدونه تبقى البطاقة خارج الشبكة، ويكتب QEMU `netdev net0 has no peer` في سجل الكونسول، ولا يوجه منفذ SSH أبدا، فيفشل كل أمر يحتاج الضيف.

سطر `-accel kvm` يسمي مسرع QEMU. بعض بناءات QEMU لا تستعمل KVM إلا حين يسمى، والضيف الذي يقع على المحاكاة البرمجية للمعالج أبطأ بكثير من أن يكون قابلا للاستعمال. يخبرك `macxvm doctor` متى يجب تسمية المسرع.

<a id="required-placeholders"></a>
## العناصر النائبة المطلوبة

يجب أن يحتوي كل ملف وسائط على هذه الخمسة:

```
{{DISK_IMG}}  {{NETDEV}}  {{MAC}}  {{RAM_MIB}}  {{CPUS}}
```

العنصر الناقص يبلغ عنه برمز الخروج 8:

```
macxvm: error: args file must contain {{DISK_IMG}} (see README: args file)
```

<a id="forbidden-flags"></a>
## الأعلام الممنوعة

‏MacXVM يضيف الأعلام التالية بنفسه، فلا يجوز أن يحتويها ملف الوسائط:

| العلم | السبب |
|-------|-------|
| `-name` | MacXVM يسمي العملية باسم الجهاز ليتعرف عليها |
| `-pidfile` | يحتاج MacXVM معرف العملية ليعرف إن كان الجهاز يعمل |
| `-qmp` | يدير MacXVM الجهاز عبر QMP |
| `-monitor` | شاشة المراقبة تتعارض مع مخرجات MacXVM |
| `-display` | يقرر MacXVM بين الوضع بلا واجهة وVNC |
| `-vnc` | يقرر MacXVM بين الوضع بلا واجهة وVNC |
| `-daemonize` | يشغل MacXVM QEMU في جلسته الخاصة |
| `-serial` | منفذ تسلسلي يتعارض مع مخرجات MacXVM |
| `-nographic` | منفذ تسلسلي يتعارض مع مخرجات MacXVM |

```
macxvm: error: args file must not contain '-monitor' (MacXVM adds it itself)
```

أغلب سكربتات الإقلاع في المشاريع الأخرى تستخدم `-monitor stdio`. احذف ذلك السطر.

<a id="injected-flags"></a>
## الأعلام المضافة

بعد وسائطك يضيف MacXVM:

```
-name guest=macxvm-<id>,process=macxvm-<id>
-pidfile <state>/vms/<name>/qemu.pid
-qmp unix:<run>/<id>.sock,server=on,wait=off
-display none
```

مع `--vnc` يتحول السطر الأخير إلى `-vnc 127.0.0.1:<n>`، حيث `<n>` أول رقم شاشة حر يبدأ من 1. منفذ الشاشة هو `5900+n` ويطبع عند تشغيل الجهاز.

<a id="adapting"></a>
## تكييف سكربت إقلاع جاهز

أصول إقلاع ضيف macOS تأتي منك. يوثق مشروعان مفتوحان كيف تحصل عليها وكيف تقلع بها:

- [OSX-KVM](https://github.com/kholia/OSX-KVM) ل kholia
- [ultimate-macOS-KVM](https://github.com/Coopydood/ultimate-macOS-KVM) ل Coopydood

‏MacXVM لا ينسخ أي محتوى منهما إلى المستودع. عند تشغيل `image prepare` صراحةً يجلب أصولًا مفصولة حسب الإصدار إلى مجلد حالتك ويتحقق من توقيعها/بصمتها؛ ولا يقبل ترخيص Apple ولا يؤتمت خطوات الحساب اليدوية. لاستخدام سكربت الإقلاع الخاص بهما:

- ‏ اتبع دليل المشروع للحصول على صورة التثبيت والمحمل والبرنامج الثابت في مجلد تختاره. هذا المجلد هو مجلد `--assets` للجهاز.
- ‏ افتح سكربت الإقلاع وخذ منه وسائط QEMU.
- ‏ احذف كل علم من [قائمة الأعلام الممنوعة](#forbidden-flags)، وأهمها `-monitor stdio`.
- ‏ استبدل القيم الثابتة بالعناصر النائبة: الذاكرة وعدد الأنوية والقرص وعنوان MAC وجهاز الشبكة.
- ‏ انقل أسطر وسيط التثبيت إلى سطر يبدأ ب `#install `، فتستخدم فقط أثناء تثبيت macOS.
- ‏ سجل الجهاز: `macxvm create NAME --args-file ./args.txt --assets ./assets`.

<a id="validation"></a>
## التحقق والأخطاء

يتحقق من ملف الوسائط مرتين: عند تسجيل الجهاز وعند تشغيله. القواعد نفسها في الحالتين.

| المشكلة | رمز الخروج | الرسالة |
|---------|-----------|---------|
| علامة اقتباس لا تغلق | 8 | `args file: unterminated quote (see README: args file)` |
| علم ممنوع | 8 | `args file must not contain '-monitor' (MacXVM adds it itself)` |
| عنصر نائب مطلوب مفقود | 8 | `args file must contain {{MAC}} (see README: args file)` |
| عنصر نائب مجهول | 8 | `args file: unknown placeholder '{{NOPE}}' (see README: args file)` |
| `{{NVRAM}}` بلا ملف NVRAM | 8 | `args file: {{NVRAM}} requires a VM with an NVRAM file (create with --nvram-template) (see README: args file)` |
| `{{ASSETS}}` بلا مجلد أصول | 8 | `args file: {{ASSETS}} requires --assets DIR (create with --assets) (see README: args file)` |

الجهاز الذي يفشل في التحقق لا ينشأ أبدا: يحذف المجلد نصف المبني من جديد.

<a id="example"></a>
## مثال عملي

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

سجل الجهاز الآن. `macxvm start demo --install --vnc` يشغله مع أسطر التثبيت وVNC. `macxvm base bootstrap demo --vnc` هو المسار الموجه للأساس الجديد، و`macxvm start demo --wait` للإقلاعات اللاحقة وينتظر SSH بالمفتاح لا QMP وحده.
