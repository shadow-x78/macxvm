<div align="center">

# تخفيف ضيف macOS - MacXVM

[![الإصدار](https://img.shields.io/badge/version-0.9.0-007AFF?style=flat-square&logo=semver)](../CHANGELOG.md)
[![الترخيص](https://img.shields.io/badge/license-MIT-1D1D1F?style=flat-square)](../LICENSE)
![Go](https://img.shields.io/badge/go-1.22%2B-00ADD8?style=flat-square&logo=go)
![المنصة](https://img.shields.io/badge/platform-Linux%20x86__64-5E5CE6?style=flat-square&logo=linux)

</div>

---

## اللغة

<a href="SLIM.md">🇬🇧 English</a> · <a href="SLIM_AR.md">🇸🇦 العربية</a>

---

## فهرس المحتويات

- [الهدف](#goal)
- [المستوى الأول](#level-1)
- [المستوى الثاني](#level-2)
- [المستوى الثالث](#level-3)
- [قائمة المنع](#deny-list)
- [ما لا يمس أبدا](#never)
- [الاستعادة](#restore)
- [القياس](#measure)
- [معيار القبول](#acceptance)
- [حل المشكلات](#troubleshooting)

---

<a id="goal"></a>
## الهدف

ضيف macOS يستهلك 30 % من المعالج في وضع الخمول لا يصلح للبناء. تقلص مستويات التخفيف في MacXVM كلفة الخمول: خدمات خلفية أقل، وفهرسة مطفأة، وحركات بلا، وتحديثات تلقائية مطفأة.

كل المستويات تعمل **داخل الضيف** عبر SSH ولا تحتاج root على المضيف. الإعدادات والخدمات المحددة قابلة للاستعادة؛ حذف التطبيقات عملية انتقائية منفصلة، والتراجع عنه يكون باستنساخ الأساس المختوم من جديد. تبقى SSH والشبكة وXcode محمية.

```bash
macxvm slim demo --level 1
macxvm slim demo --level 2 --measure
macxvm slim demo --restore
```

<a id="level-1"></a>
## المستوى الأول (آمن، وهو الافتراضي)

المستوى الأول يغير الإعدادات فقط. لا يوقف خدمة ولا يحذف ملفا:

| التغيير | الأثر |
|---------|-------|
| `defaults write NSGlobalDomain AppleInterfaceStyle Dark` | تفعيل الوضع الداكن والتحقق منه في bootstrap |
| `mdutil -a -i off` | لا فهرسة Spotlight، ولا فهرسة للقرص في الخلفية |
| `tmutil disable` | لا Time Machine، ولا لقطات متكررة |
| مؤقتات النوم في `pmset` إلى صفر | لا ينتقل الضيف إلى النوم أبدا |
| `reduceMotion` و`reduceTransparency` | حركات أقل، وعمل أقل للمركب |
| `NSAutomaticWindowAnimationsEnabled` | بلا حركات نوافذ |
| Dock بلا حركة إطلاق | يبدأ فورا |
| `CrashReporter DialogType none` | لا نوافذ انهيار تعطل الجلسة |
| `softwareupdate --schedule off` | لا تحديثات نظام تلقائية |
| `AutomaticCheckEnabled` و`AutomaticDownload` | لا يفحص macOS عن التحديثات من تلقاء نفسه |

تبقى التحديثات التلقائية مطفئة عن قصد: تحديث نظام macOS داخلguest كهذا قد يكسر الإقلاع. لا تشغل تحديثات النظام داخل الضيف.

السكربت في `resources/slim-level1.sh` مدمج في الملف التنفيذي.

<a id="level-2"></a>
## المستوى الثاني (صارم، اختياري)

المستوى الثاني يفعل كل ما في الأول، ويعطل مهام launchd المذكورة في `resources/slim-labels.txt`:

| النطاق | التسمية | وظيفتها |
|--------|---------|---------|
| `gui` | `com.apple.photoanalysisd` | تحليل مكتبة الصور |
| `gui` | `com.apple.mediaanalysisd` | تحليل مكتبة الوسائط |
| `gui` | `com.apple.suggestd` | اقتراحات النظام |
| `gui` | `com.apple.knowledge-agent` | مساعد المعرفة |
| `gui` | `com.apple.parsecd` | تحليل الأقراص |
| `gui` | `com.apple.assistantd` | المساعد |
| `gui` | `com.apple.Siri.agent` | Siri |
| `gui` | `com.apple.siriknowledged` | معرفة Siri |
| `gui` | `com.apple.helpd` | عارض المساعدة |
| `gui` | `com.apple.tipsd` | نصائح macOS |
| `system` | `com.apple.analyticsd` | تحليلات النظام |

المهمة غير الموجودة في إصدار macOS لديك تتجاهل، فتبقى القائمة صالحة بين الإصدارات. القائمة مدمجة في الملف التنفيذي، وتمر عبر سياسة الأداة قبل كتابتها في الضيف.

يمكنك توسيع الملف، لكن قائمة المنع تبقى سارية على ما تضيفه.

<a id="deny-list"></a>
## قائمة المنع

قائمة التسميات المحمية مفروضة في الشيفرة لا في الملف. التسمية التي تبدأ بأي من هذه البادئات لا تعطل أبدا:

```
com.apple.dt.                  com.apple.opendirectoryd
com.apple.CoreSimulator        com.apple.syslogd
com.openssh                     com.apple.diskarbitrationd
com.apple.launchd              com.apple.coreservices
com.apple.WindowServer         com.apple.xpc
com.apple.loginwindow           com.apple.sshd
com.apple.mDNSResponder
com.apple.networkd
com.apple.configd
com.apple.securityd
com.apple.trustd
```

هذه هي الخدمات التي تبقي SSH والشبكة وخادم النوافذ وسلسلة المفاتيح والنظام نفسه حية. المقارنة لا تراعي حالة الأحرف، ف `com.apple.sshd` و`com.apple.SSHD` محميان معا.

وحين تحذف تسمية من القائمة، تقول الأداة ذلك:

```
macxvm: warning: skipping the protected label gui com.apple.sshd
```

<a id="level-3"></a>
## المستوى الثالث (انتقائي، اختياري)

المستوى الثالث اسم توافقي لنفس محرك `macxvm apps remove --set default`. لا يحذف إلا قائمة شخصية ثابتة بعد التحقق من bundle identifier والجذور الثابتة وsymlinks وflags وSIP وsudo ونتيجة الحذف. يبقى Safari وXcode وUtilities وFinder وSystem Settings وApp Store وTerminal ومكونات الإقلاع والشبكة وكل بيانات المستخدم.

Mail وMessages وFaceTime عناصر dock-only: تراجع أو تحذف أيقونة Dock فقط، بينما يبقى التطبيق والحسابات والبريد والبيانات. لا حذف ل caches أو logs أو `/var/folders` أو simulator runtimes أو بقايا التثبيت.

<div dir="ltr" align="left">

```bash
# معاينة لا تغير الضيف
macxvm apps remove macos --set default --dry-run
# الاحتفاظ بتطبيقات محددة
macxvm apps remove macos --set default --keep "Voice Memos,Mail" --dry-run
# التنفيذ بعد المراجعة
macxvm apps remove macos --set default --yes
# اسم توافقي بنفس الحماية
macxvm slim macos --level 3 --dry-run
```

</div>

تُحفظ النتيجة في `apps-manifest.json` عند الحذف، أو `apps-dry-run.json` للمعاينة، أو `apps-list.json` للعرض، مع الحالة والسبب والمسار والحجم القابل للاستعادة بالكيلوبايت. الفشل أو الانقطاع يترك `remove-partial` بدل ادعاء النجاح.

<a id="never"></a>
## ما لا يمس أبدا

- لا يعدل مجلد النظام ولا يعطل SIP؛ حالة SIP غير المعروفة تجعل تطبيقات النظام محمية.
- لا يحذف تطبيقات النظام ولا ملفات الحسابات ولا بيانات Mail.
- لا حذف جماعي ل caches أو logs أو simulator runtimes أو بقايا التثبيت.
- فشل حذف Dock يُسجل بوضوح ولا يظهر كحذف نظيف.

الاستعادة تعيد الخدمات وSpotlight وسياسة التحديث فقط، لا التطبيقات. للتراجع عن حذف تطبيق استنسخ الأساس من جديد.

<a id="restore"></a>
## الاستعادة

```bash
macxvm slim demo --restore
```

تقرأ الاستعادة الملف `~/.macxvm-slim-disabled` داخل الضيف، وتفعل كل مهمة مذكورة فيه، وتحذف الملف، وتعيد تشغيل Spotlight ب `sudo mdutil -a -i on`، وتعيد سياسة التحديث. يجب إعادة تشغيل الضيف بعدها. لا تستعيد التطبيقات؛ استنسخ الأساس للتراجع عن حذفها.

<a id="measure"></a>
## القياس

```bash
macxvm slim demo --level 2 --measure
```

يجري القياس قبل التخفيف وبعده، مع 30 ثانية خمول بينهما، ويخزن النتيجتين في `slim-report.txt` الخاص بالجهاز:

```
=== before ===
processes: 412
top -l 2 -n 8 -o cpu -stats pid,command,cpu,mem | tail -n 12
...
vm_stat | head -6
...
memory_pressure | tail -4
...

=== after ===
processes: 380
...
```

عدد العمليات في السطر الأول أسرع طريقة لرؤية الأثر.

<a id="acceptance"></a>
## معيار القبول

يقبل التخفيف حين يتحقق كل ما يلي:

- عدد العمليات أقل من قبل،
- SSH ما زال يعمل،
- `xcodebuild -version` ما زال يعمل،
- مشروع صغير ما زال يبنى.

إن كسرت تسمية شيئا، احذفها من `resources/slim-labels.txt` وسجل التغيير في [DECISIONS.md](DECISIONS.md).

<a id="troubleshooting"></a>
## حل المشكلات

**الضيف لا يرد بعد التخفيف.**
استعده ب `macxvm slim demo --restore`، وأعد تشغيل الضيف، ثم أعد المحاولة. إن كان اتصال SSH معطوبا، فعل المهمة المعطلة من طرفية داخل الضيف:

```bash
sudo launchctl enable system/com.apple.sshd
```

**تسمية غير موجودة في قائمة الضيف.**
التسمية غير الموجودة في إصدار macOS لديك تتجاهل بصمت. تحقق من القائمة داخل الضيف ب `launchctl print system/<label>`.

**القياس لا يظهر فرقا.**
يحتاج القياس ضيفا خاملا. انتظر استقرار الضيف ثم قس من جديد. خيار `--measure` ينتظر 30 ثانية بين القياسين أصلا.
