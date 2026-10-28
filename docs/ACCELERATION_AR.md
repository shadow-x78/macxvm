# MacXVM - التسريع الأصلي

تسجل هذه الصفحة الهندسة العكسية لتعريفات الضيف، والمسرّعات الأصلية التي
يستطيع macOS تشغيلها، والأثر المقاس من تحويل الجهاز إليها، وأهداف التسريع
التي ما زالت مفتوحة.

## تعريفات الضيف هي العقد

لا يستطيع macOS تشغيل جهاز افتراضي إلا إذا كان الضيف يحمل تعريفاً له.
التعريفات امتدادات للنواة، وكل امتداد ينشر قواعد المطابقة في
`Contents/Info.plist` تحت `IOKitPersonalities`. هذه القواعد تسمّي هويات PCI
ورموز أصناف PCI وأنواع أجهزة virtio التي يطالب بها التعريف.

يقرأ MacXVM هذا الملف من الضيف الحي:

```bash
macxvm accel macos --re          # خريطة الضيف الحي
macxvm accel macos               # الخريطة مقابل الجهاز
macxvm accel macos --apply       # تحويل الجهاز إلى المسار الأصلي
```

الأمر يقرأ
`/System/Library/Extensions/<kext>.kext/Contents/Info.plist` في الضيف،
ويحوّله إلى XML عبر `plutil`، ثم يحلل الشخصيات. والأدلة أدناه من
macOS Sonoma 14.8 (البناء 23J619) على المضيف المرجعي.

## الخريطة الكاملة

### هويات PCI التي يطالب بها macOS

| هوية PCI | التعريف | الامتداد | ما يوفّره |
|---|---|---|---|
| `0x1234:0x1111` | `AppleBochVGAFB` | `AppleVirtualGraphics` | إطار ثنائي الأبعاد |
| `0x1013:0x0000` | `AppleCirrusGD5446FB` | `AppleVirtualGraphics` | إطار ثنائي الأبعاد |
| `0x106b:0xeeee` | `AppleParavirtGPUControl` | `AppleParavirtGPU` | **كرت Metal**، `IOAccelerator` و`IOFramebuffer` |
| `0x106b:0x1a00` | `AppleVirtIOStorageDevice` | `AppleVirtIOStorage` | تخزين Apple |
| `0x106b:0x1a03` | `AppleVideoToolboxParavirtualizationDriver` | `AppleVideoToolboxParavirtualization` | ترميز وفك ترميز الفيديو |
| `0x106b:0x2002` | `AppleEmbeddedNVMeController` | `IONVMeFamily` | NVMe من Apple |

### رموز أصناف PCI التي يطالب بها macOS

| رمز الصنف | التعريف | الامتداد | ما يوفّره |
|---|---|---|---|
| `0x01080200` | `IONVMeController` | `IONVMeFamily` | **تخزين NVMe** لأي بائع |

شخصية `GenericNVMeSSD` في `IONVMeFamily` تطابق رمز الصنف
`0x01080200&0xffffff00`، أي أن macOS يطالب بكل متحكم NVMe قياسي، ومنه
متحكم QEMU.

### أنواع أجهزة virtio التي يطالب بها macOS

الناقل هو ناقل virtio القياسي: شخصية `AppleVirtIOPCITransport` تطابق بائع
PCI `0x1af4`، والتعريفات التالية تطابق نوع جهاز virtio.

| نوع virtio | التعريف | ما يوفّره |
|---|---|---|
| `0x01` | `AppleVirtIONetwork` | شبكة |
| `0x02` | `AppleVirtIOBlockStorageDevice` | تخزين كتل |
| `0x03` | `AppleVirtIOConsole` | وحدة تحكم تسلسلية |
| `0x04` | `AppleVirtIOEntropy` | عشوائية |
| `0x05` | `AppleVirtIOBalloon` | نفخ ذاكرة |
| `0x09` | `AppleVirtIO9P` | مشاركة 9p |
| `0x12` (18) | `AppleVirtIOInput` | لوحة مفاتيح وفأرة ولوح تعقّب |
| `0x13` (19) | `AppleVirtIOSocket` | مقبس vsock |
| `0x19` (25) | `AppleVirtIOSound` | صوت |
| `0x1a` (26) | `AppleVirtIOFS` | مشاركة virtio-fs |

نوع كرت virtio `0x10` (16) **غير** موجود في القائمة. لا يوزّع macOS أي
تعريف لكرت virtio، فيبقى `virtio-gpu` في QEMU غائباً عن الضيف. ويبقى
العرض على `bochs-display`، وهو الإطار الذي يطالب به `AppleBochVGAFB`.

## ما الذي غيّرته الخريطة

كان المشروع يستخدم `virtio-net` ويعرف أن `vmware-svga` بلا تعريف. وتضيف
الخريطة الحقائق الناقصة:

1. **NVMe مسار أصلي.** يطالب `IONVMeFamily` كل متحكم NVMe، فلا يحتاج قرص
   النظام إلى مكدس AHCI داخل macOS.
2. **للعشوائية والنفخ تعريفات.** `AppleVirtIOEntropy` و`AppleVirtIOBalloon`
   متاحان، فيمكن كشف الجهازين.
3. **Apple توزّع كرتاً افتراضياً قادراً على Metal.** هويته `0x106b:0xeeee`
   وتعريفه `AppleParavirtGPUControl`، مع إضافة Metal `AppleParavirtGPUMetal`.
   ولا يوجد في QEMU جهاز بهذه الهوية، فلا يستطيع الضيف用它 بعد. وهو ليس
   جهاز virtio: الشخصية تربطه مباشرة بـ`IOPCIDevice`، والتعريفات المرافقة
   تطابق أسماء `paravirtualizedgraphics,gpu` و
   `paravirtualizedgraphics,iosurface`.
4. **macOS توزّع تعريفات عرض Intel.** يطالب
   `AppleIntelKBLGraphicsFramebuffer` بهويات Intel `0x8086:0x5912` و
   `0x5916` و`0x591b` و`0x591c` و`0x591e` و`0x5923` و`0x5926` و`0x5927`
   و`0x87c0`. وهي تحتاج عتاد Intel الحقيقي خلفها، فلا تصلح مساراً لمضيف
   كرته NVIDIA.

## الأثر المقاس

قياس على المضيف المرجعي بالضيف نفسه وبالحمل نفسه. الأرقام مخرجات `dd`
داخل الضيف.

| المسار | كتابة 512 MiB مع `fsync` | قراءة 512 MiB |
|---|---|---|
| AHCI (`ide-hd`، قبل) | 727,538,340 بايت/ث = 694 MiB/s | 6,963,396,568 بايت/ث = 6.6 GiB/s |
| NVMe (`nvme`، بعد) | 1,032,100,690 بايت/ث = **984 MiB/s** | 10,796,149,292 بايت/ث = **10.1 GiB/s** |

أي 1.42× في الكتابة و1.55× في القراءة. ويؤكد الضيف التعريف والميزة:

```text
$ system_profiler SPNVMeDataType
NVMExpress:
    Generic SSD Controller:
        QEMU NVMe Ctrl:
          TRIM Support: Yes
          Serial Number: macxvm
          BSD Name: disk0
```

ويظهر في `kextstat` التعريف `com.apple.iokit.IONVMeFamily (2.1.0)` محمّلاً،
والقرص هو `disk0` بخريطة أقسام GPT سليمة.Volume هو volume نفسه: انتقل
حاوية APFS وقسم EFI مع القرص.

ويُجري `macxvm accel macos --apply` هذا التحويل بالضبط:

- ينتقل قرص النظام من `-device ide-hd` إلى `-device nvme,drive=...`;
- يُضاف `-device virtio-rng-pci` لتعريف `AppleVirtIOEntropy`;
- يُضاف `-device virtio-balloon-pci` لتعريف `AppleVirtIOBalloon`;
- يبقى قرص محمّل الإقلاع على AHCI، فلا يتغير مسار إقلاع البرنامج الثابت;
- تُنسخ `args.txt` إلى `args.txt.bak-accel-<وقت>` قبل الكتابة.

## كرت الشاشة على هذا العتاد

المضيف فيه جهاز عرض واحد، وهو NVIDIA GeForce RTX 5060، والضيف هو macOS.
وليس في macOS أي تعريف لأي كرت NVIDIA GeForce: أنهت Apple الدعم في 2019،
وآخر تعريفاتها تتوقف عند Kepler. فسيظهر الكرت الممرَّر في الضيف جهاز PCI
مجهولاً بلا تسريع، ويزيل جهاز العرض الوحيد العامل على المضيف.

لكن MacXVM يهيّئ المضيف للتمرير، لأن ذلك هو الجزء الذي يتحكم فيه المضيف:

```bash
macxvm gpu macos --passthrough              # ما ينقص المضيف
macxvm gpu macos --enable-iommu             # تفعيل VT-d في سطر أوامر النواة
macxvm gpu macos --bind-vfio 0000:01:00.0   # تسليم الكرت إلى vfio-pci
```

يضيف `--enable-iommu` الخيارين `intel_iommu=on iommu=pt` إلى كل مدخل
نواة عبر `grubby` ويطلب إعادة التشغيل. و`--bind-vfio` يفكّ تحميل وحدات
NVIDIA ويفكّ ربط الكرت ويربطه بـ`vfio-pci`. ويرفض الأمر ربط جهاز العرض
الوحيد على المضيف، لأن ذلك يزيل سطح مكتب المضيف نفسه.

أما جانب الضيف في التمرير فمسألة أخرى، والجواب في التعريفات: macOS يطالب
بـ`0x106b:0xeeee` لكرته الافتراضي الخاص، ولا يطالب بأي شيء آخر يسرّع.
فهدف التسريع المفتوح هو تنفيذ هذا الجهاز في QEMU بالبروتوكول الذي يتحدثه
`AppleParavirtGPU`. وهو ليس virtio-gpu، فلا يمكن إعادة استخدام أي جهاز
موجود في QEMU له.

## الأوامر

```bash
macxvm accel macos --re               # خريطة الهندسة العكسية للضيف
macxvm accel macos                    # الخريطة مقابل الجهاز
macxvm accel macos --check            # التقرير، مع فشل عند وجود مكسب ناقص
macxvm accel macos --apply            # جهاز متوقف: التحويل إلى المسرّعات الأصلية
macxvm accel macos --json             # التقرير نفسه بصيغة JSON
macxvm gpu macos --passthrough        # جاهزية المضيف للتمرير
```

## خلاصة صريحة

- تعريفات الضيف هي المصدر، وMacXVM يقرأها.
- macOS يشغّل NVMe أصلياً، وقرص النظام صار عليه: 1.42× في الكتابة و
  1.55× في القراءة، مع TRIM.
- macOS يشغّل ناقل virtio القياسي لعشرة أنواع أجهزة، وكُشف العشوائية
  والنفخ الآن.
- لا يوجد في macOS تعريف لـvirtio-gpu، فيبقى العرض على الإطار ثنائي
  الأبعاد في Bochs.
- الجهاز الوحيد القادر على Metal الذي يطالب به macOS هو جهاز Apple الخاص
  `0x106b:0xeeee`، ولا ينفّذه أي جهاز في QEMU بعد.
- كرت NVIDIA في هذا المضيف لا يستطيع تسريع macOS مهما فعل المضيف.
