<div align="center">

# بناء المشاريع - MacXVM

[![الإصدار](https://img.shields.io/badge/version-0.5.0-007AFF?style=flat-square&logo=semver)](../CHANGELOG.md)
[![الترخيص](https://img.shields.io/badge/license-MIT-1D1D1F?style=flat-square)](../LICENSE)
![Go](https://img.shields.io/badge/go-1.22%2B-00ADD8?style=flat-square&logo=go)
![المنصة](https://img.shields.io/badge/platform-Linux%20x86__64-5E5CE6?style=flat-square&logo=linux)

</div>

---

## اللغة

<a href="BUILDING.md">🇬🇧 English</a> · <a href="BUILDING_AR.md">🇸🇦 العربية</a>

---

## فهرس المحتويات

- [نظرة عامة](#overview)
- [كشف نوع المشروع](#detection)
- [ملف macxvm.json](#project-file)
- [مجلد العمل](#workdir)
- [المزامنة](#sync)
- [أمر البناء](#build-command)
- [جلب النتائج](#fetch)
- [البناء المؤقت](#ephemeral)
- [سير العمل up وdown](#up-down)
- [توقيع الشيفرة](#signing)
- [الملفات التعريفية](#profiles)

---

<a id="overview"></a>
## نظرة عامة

يبني `macxvm build` المشروع داخل ضيف: يُنسخ المشروع عبر SSH، ويجري البناء هناك، ويُبثّ المخرجات إلى الطرفية. رمز خروج البناء هو رمز خروج الأمر.

```bash
macxvm build --project . --scheme App
macxvm build --vm work --project . --scheme App --configuration Release
macxvm build --ephemeral --from base-xcode --project . --scheme App
```

لا يُشغَّل الجهاز إلا إن لم يكن يعمل، ويُوقف من جديد بعد ذلك حتى عند فشل البناء. ويبقيه `--keep` يعمل.

<a id="detection"></a>
## كشف نوع المشروع

يُكشف نوع المشروع في المستوى الأعلى من مجلد المشروع:

| ما الموجود | النوع | البناء |
|------------|-------|--------|
| `*.xcworkspace` | xcode | `xcodebuild -workspace ...` |
| `*.xcodeproj` | xcode | `xcodebuild -project ...` |
| `Package.swift` | swiftPackage | `swift build` |
| لا شيء من ذلك | لا ينطبق | خطأ `unsupported project` |

يُتجاهل ملف `.xcodeproj` داخل مساحة عمل، وتُقدَّم مساحة العمل على المشروع.

يحتاج مشروع من نوع `xcode` جهازًا من الملف التعريفي `xcode`. وجهاز الملف التعريفي `cli` لا يحتوي Xcode، فيتوقف البناء برمز الخروج 9:

```
macxvm: error: this profile has no Xcode; use profile 'xcode'
```

<a id="project-file"></a>
## ملف macxvm.json

يضع ملف `macxvm.json` في جذر المشروع القيم الافتراضية:

```json
{
  "version": 1,
  "profile": "xcode",
  "vm": "dev",
  "base": "base-xcode",
  "build": {
    "scheme": "App",
    "configuration": "Debug",
    "destination": "platform=iOS Simulator,name=iPhone 16"
  },
  "sync": { "exclude": ["node_modules"] },
  "fetch": [ { "remote": "build/App.app", "local": "./out/App.app" } ]
}
```

| المفتاح | المعنى |
|---------|--------|
| `version` | مطلوب، ويجب أن يكون `1` |
| `profile` | `cli` أو `xcode` |
| `vm` | الجهاز الذي يجري فيه البناء |
| `base` | صورة الأساس الختم التي يُنسخ منها الجهاز |
| `build.scheme` | السكيم المطلوب بناؤه |
| `build.configuration` | `Debug` أو `Release` |
| `build.destination` | وجهة البناء |
| `sync.exclude` | مسارات إضافية لا تُرسل إلى الضيف أبدًا |
| `fetch` | المسارات التي تُنسخ بعد نجاح البناء |

كل مفتاح عدا `version` اختياري، والمفاتيح المجهولة تُتجاهل. أعلام سطر الأوامر تغلب على الملف دائمًا.

يكتب `macxvm init` ملفًا أدنى، ويخمّن الملف التعريفي من المشروع:

```bash
macxvm init          # يكتب {"version": 1, "profile": "xcode"}
macxvm init --force  # يستبدل ملفًا موجودًا
```

<a id="workdir"></a>
## مجلد العمل

يُنسخ المشروع إلى مجلد في الضيف مشتق من مسار المشروع:

```
~/macxvm-work/<أول 12 حرفًا من SHA-1 للمسار المطلق للمشروع>
```

المشروع نفسه ينتهي دائمًا في المجلد نفسه، فتُعاد данные المشتقة وذاكرة البناء المؤقتة بين البناءات.

<a id="sync"></a>
## المزامنة

```bash
rsync -a --delete \
  --exclude .git --exclude DerivedData --exclude build --exclude .build \
  -e "ssh <خيارات MacXVM>" \
  <project>/ admin@127.0.0.1:~/macxvm-work/<hash>/
```

- `-a` بلا `-z`: الملفات مضغوطة أصلًا على قرص الضيف، وضغط الشبكة يكلّف أكثر مما يوفّر.
- المسارات المستثناة محميّة من `--delete`، فتبقى `DerivedData` و`.build` داخل الضيف بين عمليتي مزامنة.
- تُضاف استثناءات `macxvm.json` إلى القائمة.

<a id="build-command"></a>
## أمر البناء

لمشروع Xcode:

```
cd ~/macxvm-work/<hash> && xcodebuild \
  [-workspace <workspace> | -project <project>] \
  -scheme <scheme> -configuration <configuration> \
  [-destination <destination>] \
  -jobs <cpus> \
  -derivedDataPath ~/macxvm-work/<hash>/DerivedData \
  COMPILER_INDEX_STORE_ENABLE=NO \
  [ONLY_ACTIVE_ARCH=YES] \
  build [الوسائط الإضافية]
```

- `-jobs` يأخذ عدد أنوية الضيف افتراضيًا.
- `COMPILER_INDEX_STORE_ENABLE=NO` يتخطى فهرس المُصرِّف، وهو أكبر مصدر لهدر في بناء لا يقرأه أحد.
- `ONLY_ACTIVE_ARCH=YES` يُضاف في بناء `Debug` ليُصرَّف المعمارية الحالية وحدها.
- بدون `--scheme` يُسأل الضيف عن سكيماته. المشروع ذو السكيم الواحد يستخدمه، وذو السكيمات العدة يتوقف ويطلب `--scheme`.

لحزمة Swift:

```
cd ~/macxvm-work/<hash> && swift build -c <configuration> --jobs <cpus> [الوسائط الإضافية]
```

تأتي الوسائط الإضافية من سطر الأوامر بعد `--`:

```bash
macxvm build --project . --scheme App -- -quiet
```

<a id="fetch"></a>
## جلب النتائج

```bash
macxvm build --project . --scheme App --fetch build/App.app=./out/App.app
```

تُنسخ النتيجة بـ `scp -r` بعد نجاح البناء. ويجب أن يكون المساران نسبيين وأن يبقيا داخل المشروع: قائمة `fetch` في مستودع مستنسخ يجب ألا تكتب في أي مكان آخر على المضيف.

تُحترم قائمة `fetch` في `macxvm.json` كذلك.

<a id="ephemeral"></a>
## البناء المؤقت

```bash
macxvm build --ephemeral --from base-xcode --project . --scheme App
```

يجري البناء في طبقة مؤقتة فوق صورة الأساس الختم. تُنشأ الطبقة وتُشغَّل وتُستخدم وتُحذف من جديد، حتى عند فشل البناء أو مقاطعة الأمر. ويستخدم الجهاز وضع الذاكرة المخزّنة السريع غير الآمن، وهو مسموح للجهاز المؤقت فقط.

يحفظ `--keep` الجهاز لتفحصه.

<a id="up-down"></a>
## سير العمل up وdown

```bash
macxvm up      # ينسخ الأساس إن لزم، ويشغّل الجهاز، وينتظر SSH
macxvm down    # يوقف الجهاز بلطف
macxvm down --delete   # يوقفه ويحذف الطبقة
```

ينشئ `macxvm up` الجهاز من `base` في `macxvm.json` إن لم يكن موجودًا، ثم يشغّله. و`macxvm down --delete` لا يحذف إلا جهازًا نُسخ من ذلك الأساس، وكل ما عداه يحتاج `--yes`.

<a id="signing"></a>
## توقيع الشيفرة

يحتاج توقيع الشيفرة عبر SSH سلسلة مفاتيح غير مقفلة. للبناء الذي لا يحتاج توقيعًا استخدم وجهة محاكي أو `CODE_SIGNING_ALLOWED=NO`:

```bash
macxvm build --project . --scheme App --destination "platform=iOS Simulator,name=iPhone 16"
```

لفتح سلسلة المفاتيح داخل الضيف نفّذ فيه:

```bash
security unlock-keychain
```

الضيف من معمارية x86_64، فيُنتج بناء المحاكي محاكيًا بمعمارية x86_64. أما بناء جهاز حقيقي فيُصرَّف بشكل طبيعي، لكنه لا يعمل في محاكي الضيف.

<a id="profiles"></a>
## الملفات التعريفية

| الملف | المحتوى | `swift build` | `xcodebuild` | حزمة iOS |
|-------|---------|---------------|--------------|---------|
| `cli` | macOS خفيف + أدوات سطر الأوامر | نعم | لا (رمز الخروج 9) | لا |
| `xcode` | macOS خفيف + أدوات سطر الأوامر + Xcode | نعم | نعم | نعم، بعرض برمجي |

أدوات سطر الأوامر تحتوي حزمة macOS فقط. لذلك الملف `cli` لحزم Swift وHomebrew وgit والسكربتات.
