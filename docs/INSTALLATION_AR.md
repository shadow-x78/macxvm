<div align="center">

# دليل التثبيت - MacXVM

[![الإصدار](https://img.shields.io/badge/version-0.8.0-007AFF?style=flat-square&logo=semver)](../CHANGELOG.md)
[![الترخيص](https://img.shields.io/badge/license-MIT-1D1D1F?style=flat-square)](../LICENSE)
![Go](https://img.shields.io/badge/go-1.22%2B-00ADD8?style=flat-square&logo=go)
![المنصة](https://img.shields.io/badge/platform-Linux%20x86__64-5E5CE6?style=flat-square&logo=linux)

</div>

---

## اللغة

<a href="INSTALLATION.md">🇬🇧 English</a> · <a href="INSTALLATION_AR.md">🇸🇦 العربية</a>

---

## فهرس المحتويات

- [المتطلبات](#requirements)
- [البناء والتثبيت](#build)
- [التثبيت اليدوي](#manual)
- [التحقق](#verify)
- [مجلد الحالة](#state)
- [التحديث](#updating)
- [إلغاء التثبيت](#uninstall)
- [التوزيع](#distribution)

---

<a id="requirements"></a>
## المتطلبات

| المتطلب | التفصيل |
|----------|---------|
| المضيف | Linux بمعمارية x86_64 مع المحاكاة الافتراضية العتادية (Intel VT-x أو AMD-V) |
| KVM | `qemu-system-x86_64` بإصدار 6.0 أو أحدث و`qemu-img`، و`/dev/kvm` يستطيع مستخدمك استعماله |
| أدوات المضيف | `ssh` و`scp` و`rsync` و`ssh-keygen` و`ssh-keyscan` و`sshpass` (بديل كلمة المرور في bootstrap) و`python3` (للقطات) |
| أدوات الأتمتة | `tesseract` و`vncdo` و`genisoimage` أو `mkisofs` لأمر `autoinstall` |
| العارض | `vncviewer` أو `gvncviewer` أو `remmina` أو `xdg-open` مع معالج VNC؛ وإلا يطبع `open` عنوان loopback الفعلي |
| Go | 1.22 أو أحدث، لبناء MacXVM فقط |
| الضيف | macOS بمعمارية x86_64 ≤ 26، تثبته أنت |

‏MacXVM ملف تنفيذي واحد ثابت بلا خادم دائم. لا يحتاج من المضيف إلا الأدوات أعلاه، ولا يعمل أبدا بصلاحيات root.

```bash
macxvm doctor
```

<a id="build"></a>
## البناء والتثبيت

```bash
git clone https://github.com/shadow-x78/macxvm.git ~/MacXVM
cd ~/MacXVM && ./scripts/install.sh
```

يبني السكربت الملف التنفيذي ويثبته في `~/.local/bin`. عين `PREFIX` لتثبيته في مكان آخر:

```bash
PREFIX=/usr/local/bin ./scripts/install.sh
```

<a id="manual"></a>
## التثبيت اليدوي

```bash
git clone https://github.com/shadow-x78/macxvm.git && cd macxvm
make build
make install
```

| الهدف | ما يفعله |
|-------|-----------|
| `make build` | يبني ملفا تنفيذيا ثابتا ل `linux/amd64` في `bin/macxvm` |
| `make test` | يشغل `go vet ./...` و`go test ./...` |
| `make install` | يبني ويثبت الملف التنفيذي في `~/.local/bin` |
| `make clean` | يحذف `bin/` |

الملف التنفيذي ثابت ويبنى ب `CGO_ENABLED=0` فلا يحتاج مكتبة C على المضيف. إصدارات الإصدار تستخدم ldflags في Makefile، وحد CI هو 8 MiB. الملف الافتراضي `cli` يحتاج 4096 MiB RAM وقرصًا منطقيًا 64 GiB على الأقل، والمساحة الفعلية على المضيف تغطي البيانات المخصصة.

<a id="verify"></a>
## التحقق

```bash
macxvm version
macxvm doctor
```

يفحص `macxvm doctor` نظام التشغيل و`/dev/kvm` وعلامات المحاكاة الافتراضية في المعالج وQEMU ووحدة KVM وأدوات المضيف ومساحة القرص الحرة وذاكرة المضيف والأجهزة المسجلة. وينتهي بالتنببيه القانوني، لأن تشغيل macOS تحت QEMU/KVM على حاسوب مكتبي شأنك أنت لا شأن الأداة.

<a id="state"></a>
## مجلد الحالة

كل ما يعرفه MacXVM موجود في مجلدين:

| المجلد | الافتراضي | المحتوى |
|--------|-----------|---------|
| `$MACXVM_HOME` | `$XDG_DATA_HOME/macxvm`، وإلا `~/.local/share/macxvm` | أصول مفصولة حسب الإصدار، والأجهزة وأقراصها والإعدادات و`known_hosts` ومفتاح SSH |
| `$MACXVM_RUN` | `$XDG_RUNTIME_DIR/macxvm`، وإلا `/tmp/macxvm-<uid>` | مقابيس QMP للأجهزة العاملة |

يمكن تغيير كليهما بمتغيري البيئة نفسيهما. ويجب أن يكون مجلد التشغيل مجلدا حقيقيا للمستخدم الحالي لا يستطيع غيره الكتابة فيه، لأن مقبس QMP داخله بلا مصادقة.

```bash
export MACXVM_HOME=/srv/macxvm
export MACXVM_RUN=/run/user/$(id -u)/macxvm
```

<a id="updating"></a>
## التحديث

```bash
cd ~/MacXVM
git pull
./scripts/install.sh
```

الأجهزة نفسها موجودة في مجلد الحالة ولا يمسها التحديث.

<a id="uninstall"></a>
## إلغاء التثبيت

```bash
rm -f ~/.local/bin/macxvm
rm -rf ~/.local/share/macxvm
```

يحذف الأمر الثاني كل الأجهزة المسجلة والأقراص وmanifests الأصول ومفتاح SSH. انسخ مجلد الحالة أولًا إن أردت الاحتفاظ بالأجهزة. لا تحذف أساسًا له طبقات؛ استخدم `macxvm list` و`macxvm delete` وفحوص الاعتماد.

<a id="distribution"></a>
## التوزيع

لا يوجد حتى الآن مستودع حزم ل MacXVM. تحمل صفحة الإصدارات الملف التنفيذي الثابت ل `linux/amd64`، وتبنى حزمة التوزيع من هدف `make build` نفسه.

يجب أن تحافظ حزمة التوزيع على هذه الخصائص:

- الملف التنفيذي ثابت ومبني ب `CGO_ENABLED=0`،
- الإصدار مبني ل `linux/amd64`،
- لا تضم أي برمجيات من Apple ولا برامج ثابتة ولا محملات إقلاع ولا مفاتيح أبدا،
- يبقى التنبيه القانوني في README وفي `macxvm doctor` كما هو.
