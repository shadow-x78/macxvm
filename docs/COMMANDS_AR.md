# مرجع الأوامر

كل أوامر `macxvm` مع الأعلام التي تقبلها. صفحة [README](../README_AR.md)
تعرض الأوامر القليلة التي ستستخدمها فعلا؛ هذه الصفحة هي المرجع.

اكتب `macxvm <command> --help` لتعني المعلومات نفسها من الأداة نفسها،
و`macxvm` وحدها لقائمة الأوامر المجمّعة.

---

## المحتويات

- [الأجهزة](#الأجهزة)
- [أصول الإقلاع](#الأصول)
- [الضيف](#الضيف)
- [المشاريع](#المشاريع)
- [المضيف](#المضيف)
- [رموز الخروج](#رموز-الخروج)

---

<a id="الأجهزة"></a>
## الأجهزة

| الأمر | ما يفعله |
|-------|----------|
| `macxvm create NAME (--base BASE \| --args-file FILE)` | يسجل جهازا افتراضيا |
| `macxvm list [--json]` | يعرض الأجهزة المسجلة |
| `macxvm status NAME [--json]` | يعرض حالة جهاز |
| `macxvm start NAME [--display install\|daily\|recovery] [--vnc] [--wait] [--force]` | يشغل الجهاز |
| `macxvm stop NAME [--force] [--timeout SECONDS]` | يوقف الجهاز |
| `macxvm open NAME [--restart]` | يفتح شاشة VNC على الواجهة المحلية دون انتظار SSH |
| `macxvm resize NAME [--cpus N] [--ram SIZE] [--disk GiB] [--force]` | يغير موارد جهاز متوقف؛ والقرص يكبر فقط |
| `macxvm autoinstall NAME --yes [--accept-license] [--timeout MINUTES]` | يثبت macOS عبر Recovery ويتوقف عند أول صفحة في Setup Assistant |
| `macxvm finish NAME [--password-stdin] [--apps] [--keep APP ...] [--seal]` | بعد Setup Assistant: مفتاح SSH وsudo والوضع الداكن والتخفيف والعرض، وختم اختياري |
| `macxvm optimize NAME [--check] [--width N] [--height N]` | يحوّل جهازا متوقفا إلى عرض Bochs |
| `macxvm accel NAME [--check] [--apply] [--re] [--json]` | يقارن الجهاز بالأجهزة التي يقودها macOS أصلا |
| `macxvm gpu [NAME] [--status] [--json]` | يعرض حالة GPU للمضيف أو لجهاز |
| `macxvm migrate NAME --to USER@HOST [--check\|--live] [--dest NAME]` | يفحص ترحيل جهاز أو ينفّذه |
| `macxvm migrate NAME --cancel` | يلغي ترحيل جار |
| `macxvm delete NAME [--yes]` | يحذف جهازا |
| `macxvm clone SRC DST [--full]` | ينسخ جهازا؛ copa افتراضيا كطبقة فوق أساس ختم |
| `macxvm compact NAME` | يضغط قرص جهاز متوقف |
| `macxvm prune [--all]` | يحذف بقايا الأجهزة المؤقتة |

### إنشاء جهاز

```bash
macxvm create NAME --base BASE
macxvm create NAME --args-file FILE [--assets DIR] [--nvram-template FILE]
```

`--base` و`--args-file` هما الطريقتان لوصف جهاز، ولا تُجمعان مع الأعلام التي
تغيّر عتاد الجهاز: `--cpus` و`--ram` و`--disk` و`--profile` و`--cpu`
و`--gpu-backend` و`--gpu` و`--audio` و`--host-gpu` وغيرها. نسخة من أساس
ترث عتاد الأساس.

| العلم | المعنى |
|-------|--------|
| `--profile cli\|xcode` | ملف الموارد: `cli` هو 6 vCPU و12 غيغابايت وقرص 64 غيغابايت، و`xcode` هو 4 vCPU و8 غيغابايت وقرص 100 غيغابايت |
| `--cpus N` | عدد المعالجات الافتراضية |
| `--ram SIZE` | الذاكرة بلاحقة: `8G` أو `12GiB` أو `16384M` |
| `--disk GiB` | حجم القرص المنطقي |
| `--user NAME` | المدير المحلي في الضيف، و`admin` افتراضيا |
| `--cpu MODEL` | نموذج المعالج في الضيف، و`skylake` افتراضيا |
| `--macos-version V` | إصدار macOS، يؤخذ من الأصول إن لم يُمرَّر |
| `--replace` | يستبدل جهازا متوقفا بنفس الاسم |
| `--force` | يتجاوز حدود المضيف؛ ولا يستبدل جهازا موجودا أبدا |

### GPU والعرض

هذه هي الأوامر التجريبية التي تعطي الضيف بطاقة رسوميات حقيقية.

```bash
macxvm gpu                                      # المضيف: IOMMU والبطاقات والعرض
macxvm gpu NAME                                 # جهاز: التعريف والkext والجاهزية
macxvm gpu NAME --status                        # نفس ذلك، بتقرير قصير
macxvm gpu NAME --passthrough                   # ما يحتاجه المضيف لتمرير بطاقة
macxvm gpu NAME --enable-iommu                  # يشغل VT-d في سطر أوامر النواة
macxvm gpu --prepare-vfio --gpu BDF --audio BDF --host-gpu BDF --accept-host-changes
macxvm gpu --unbind-vfio --gpu BDF --restore-host
macxvm gpu NAME --install-driver PACKAGE --accept-driver-changes
macxvm gpu NAME --remove-driver
macxvm gpu NAME --recover
```

التفاصيل والأرقام المقيسة وحدود التمرير في [GPU.md](GPU_AR.md).

### حزمة التعريف

```bash
macxvm driver verify PACKAGE         # يفحص التوقيع والبصمات
macxvm driver info PACKAGE [--json]   # يعرض محتوى الحزمة
macxvm driver build --builder USER@HOST
```

الصيغة وجذور الثقة وبناء الحزمة في [DRIVER.md](DRIVER_AR.md).

<a id="الأصول"></a>
## أصول الإقلاع

| الأمر | ما يفعله |
|-------|----------|
| `macxvm image link` | يعرض مصادر أصول الإقلاع دون تنزيل شيء |
| `macxvm image prepare NAME [--macos VERSION] [--profile cli\|xcode]` | يجلب أصول الإقلاع ويسجل الجهاز |

```bash
macxvm image prepare macos --profile cli
```

تنزّل الأداة نحو 1 غيغابايت إلى مجلد الحالة: مثبت macOS من خدمة الاسترجاع
لـApple، ومحمل الإقلاع OpenCore، والبرامج الثابتة لـQEMU. كل ملف يُفحص
مقابل بصمة بيانه قبل استخدامه، ولا يُخزَّن شيء من Apple في هذا المستودع.

أسماء macOS المدعومة: `high-sierra` و`mojave` و`catalina` و`big-sur` و
`monterey` و`ventura` و`sonoma` و`sequoia` و`tahoe`.

<a id="الضيف"></a>
## الضيف

| الأمر | ما يفعله |
|-------|----------|
| `macxvm ssh NAME` | جلسة SSH تفاعلية |
| `macxvm ssh-key [--install NAME]` | يعرض مفتاح SSH أو يثبته في ضيف |
| `macxvm exec NAME -- CMD ...` | ينفّذ أمرا؛ و码코드 الخروج يُمرَّر كما هو |
| `macxvm cp SRC DST` | ينسخ ملفات؛ ومسار الضيف بصيغة `NAME:/path` |
| `macxvm share add NAME --host DIR [--guest DIR]` | يشارك مجلدا بين المضيف والضيف |
| `macxvm share list NAME` | يعرض المجلدات المشاركة |
| `macxvm share sync NAME [--id ID] [--watch]` | ينسخ في الاتجاهين ويبقي أحدث ملف |
| `macxvm share push NAME [--id ID] [--delete]` | ينسخ مجلد المضيف إلى الضيف |
| `macxvm share pull NAME [--id ID] [--delete]` | ينسخ مجلد الضيف إلى المضيف |
| `macxvm share remove NAME --id ID` | يوقف المشاركة ويحتفظ بالملفات |
| `macxvm slim NAME [--level 1\|2\|3] [--keep APP ...] [--dry-run] [--yes] [--measure] [--restore]` | يخفّف الجهاز |
| `macxvm apps list NAME` | يعرض تطبيقات الضيف |
| `macxvm apps remove NAME --set default [--dry-run] [--yes]` | يحذف التطبيقات المسموح بها |
| `macxvm tools install NAME clt\|brew` | يثبت أدوات سطر الأوامر أو Homebrew |
| `macxvm tools brew NAME PKG ...` | يثبت حزم Homebrew |
| `macxvm tools list NAME` | يعرض الأدوات المثبتة |
| `macxvm xcode install NAME --xip PATH [--platform iOS]` | يثبت Xcode |
| `macxvm xcode clean NAME` | يحذف ذاكرة Xcode المؤقتة |

```bash
macxvm exec macos -- swift --version
macxvm exec macos -- sh -c 'cd /tmp && ls'
macxvm cp ./local.txt macos:/Users/admin/remote.txt
macxvm cp macos:/Users/admin/remote.txt ./local.txt
```

المزامنة في الاتجاهين تحتفظ بأحدث ملف من الاثنين ولا تحذف أي ملف، لأن الملف
الذي لا يملكه طرف قد يكون ملفا جديدا عند الطرف الآخر. لا يُنسخ الحذف إلا
عند تشغيل `push` أو `pull` مع `--delete`.

<a id="المشاريع"></a>
## المشاريع

| الأمر | ما يفعله |
|-------|----------|
| `macxvm init [--profile P] [--force]` | يكتب ملف `macxvm.json` في مجلد المشروع |
| `macxvm up` | ينشئ جهاز المشروع ويشغله |
| `macxvm down [--delete] [--yes]` | يوقف جهاز المشروع |
| `macxvm build [--vm NAME] [--project DIR] [--scheme S] [--configuration Debug\|Release] [--destination D] [--ephemeral --from BASE] [--keep] [--fetch REMOTE=LOCAL] [--jobs N] [-- ARGS]` | يبني مشروعا في ضيف |
| `macxvm run --from BASE [--cpus N] [--ram SIZE] [--project DIR] [--fetch REMOTE=LOCAL] [--keep] -- CMD ...` | ينفّذ أمرا في جهاز مؤقت |

```bash
macxvm init --profile cli
macxvm up
macxvm build --scheme MyApp
macxvm down --delete --yes
```

ملف المشروع ومسار البناء في [BUILDING.md](BUILDING_AR.md).

<a id="المضيف"></a>
## المضيف

| الأمر | ما يفعله |
|-------|----------|
| `macxvm doctor` | يفحص إن كان هذا المضيف يستطيع تشغيل ضيوف macOS |
| `macxvm version` | يعرض إصدار macxvm |
| `macxvm completion bash\|zsh` | يطبع نص إكمال الأوامر |

```bash
macxvm doctor
macxvm completion zsh > ~/.zsh/completions/_macxvm
```

<a id="رموز-الخروج"></a>
## رموز الخروج

| الرمز | المعنى |
|-------|--------|
| 0 | نجاح |
| 1 | خطأ |
| 2 | استخدام خاطئ |
| 3 | غير موجود |
| 4 | يعمل بالفعل |
| 5 | لا يعمل |
| 6 | مضيف غير مدعوم |
| 7 | انتهاء المهلة |
| 8 | ملف وسائط غير صالح |
| 9 | عدم تطابق الملف التعريفي أو الأدوات |

كل رسالة خطأ على stderr تسمّي الأمر الذي يصلحها.

---

كل مستند في هذا المشروع موجود بالإنجليزية وبالعربية: [المرجع
العربي](COMMANDS_AR.md) يقابل هذه الصفحة.
