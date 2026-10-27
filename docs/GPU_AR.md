# MacXVM - العرض وكرت الشاشة

تسجل هذه الصفحة ما يبلغ عنه الضيف فعليًا، والهندسة العكسية وراء العرض الافتراضي،
والحدود التي لا يمكن تجاوزها بالتهيئة.

## ما يراه macOS الآن

شغّل `macxvm gpu NAME` والجهاز يعمل. على مضيف هذا المشروع يبلغ الضيف عن:

| الحقل | القيمة |
|---|---|
| Vendor ID | `0x1234` |
| Device ID | `0x1111` |
| الصنف | `AppleBochVGAFB` |
| Kext | `AppleBochVGAFB` داخل `AppleVirtualGraphics.kext` |
| الناقل | PCIe بعرض x1 |
| ذاكرة الإطار | `IOFBMemorySize = 8294400` بايت |
| ذاكرة الإطار ÷ 4 بايت لكل بكسل | 2,073,600 بكسل = 1920x1080 بالضبط |

أي أن `8294400 = 1920 × 1080 × 4`، فmacOS لا يبلغ عن كرت مبتور بذاكرة 7 MB،
بل عن إطار واحد كامل بدقة 1080p. سطر `VRAM (Total): 7 MB` في معلومات النظام هو
هذا الإطار الواحد، وليس سعة QEMU البالغة 256 MB.

## الهندسة العكسية لتعريفات الضيف

التعريف الذي يقود شاشة افتراضية في macOS هو
`/System/Library/Extensions/AppleVirtualGraphics.kext`، وقائمة شخصياته تطابق
زوجين فقط من PCI:

| Vendor | Device | صنف التعريف |
|---|---|---|
| `0x1234` | `0x1111` | `AppleBochVGAFB` |
| `0x1013` | `0x0000` | `AppleCirrusGD5446FB` |

ولا شيء غيرهما. ولا يوجد في macOS تعريف `virtio-gpu`: عائلات VirtIO الموجودة هي
`AppleVirtIO` و`AppleVirtIOStorage` و`AppleVirtIOBiometrics`
و`AppleVirtIONeuralEngineDevice` فقط. و`AppleParavirtIOSurface.kext` موجود لكنه
يخص Virtualization.framework من Apple على مضيف macOS، فلا يفيد من QEMU على Linux.

لهذا فإن الافتراضي هو `-vga none` مع `bochs-display`: هو الجهاز الافتراضي الوحيد
الذي يملك macOS له تعريف إطار حقيقي. أما `vmware-svga` فكان ينتج
`No Kext Loaded`، أي بلا تعريف إطلاقًا.

## لماذا لا يوجد Metal ولا تسريع ثلاثي الأبعاد

`AppleBochVGAFB` تعريف إطار ثنائي الأبعاد. لا يوفّر خدمة `IOAccelerator` لكرت
رسومي، لذلك Metal وتركيب Core Animation على GPU ومحاكي iOS كلها تعود إلى العرض
البرمجي على المعالج. هذا حد قدرات التعريف لا خطأ تهيئة، ولا يغيّره أي علم في QEMU.

## لماذا تمرير NVIDIA ليس حلًا هنا

يعرض `macxvm gpu NAME` حقائق المضيف. على المضيف المرجعي:

```text
0000:01:00.0 VGA compatible controller: NVIDIA Corporation GB206 [GeForce RTX 5060]
IOMMU groups: 0
```

عائقان مستقلان:

1. **لا IOMMU.** لا توجد أي IOMMU groups، ولا `intel_iommu=on` أو `iommu=pt` في
   سطر أوامر النواة، والمعالج بلا كرت مدمج (لاحقة `F` تعني تعطيل الرسوميات).
   تمرير جهاز يحتاج تفعيل VT-d في البرنامج الثابت، و`intel_iommu=on iommu=pt` في
   سطر أوامر النواة، وربط الكرت بـ`vfio-pci`.
2. **لا تعريف في macOS.** حتى مع IOMMU، لا يملك macOS تعريفًا لـRTX 5060
   (معمارية Blackwell). آخر تعريفات NVIDIA لماك تتوقف عند Kepler. الكرت الممرَّر
   سيظهر كجهاز PCI مجهول بلا تسريع، وسيزيل جهاز العرض الوحيد العامل.

لذلك التمرير ليس مسار أداء على هذا العتاد، وMacXVM لا يدّعي غير ذلك.

## ما يجعل الضيف أسرع فعلًا

قياسات على المضيف المرجعي بنفس الحمل داخل الضيف:

| التغيير | الأثر |
|---|---|
| `+avx2,+fma,+bmi1,+bmi2,+sha-ni` في نموذج المعالج | CPUID في الضيف يعرض الآن AVX2 وFMA وBMI1/2 وSHA |
| من 4 إلى 8 vCPU | حمل متوازٍ نزل من 18.4 ثانية إلى 6.3 ثانية، نحو 2.9× |
| من 8 إلى 12 GiB ذاكرة | لا swap داخل الضيف في الخمول؛ `vm_stat` يظهر صفر swapins |
| `slim --level 1` | تقليل الحركة والشفافية وحركات النوافذ وDock |
| إطار 1920x1080 | الافتراضي؛ الدقة الأعلى تعني بكسلًا أكثر على عارض برمجي |
| `-cpu host` | **يفشل في الإقلاع** على هذا المضيف؛ لا تستخدمه |

و`macxvm optimize NAME --width 1440 --height 900` يقايض الحدة ببكسل أقل إن بقي
الضيف بطيئًا. الإعداد يطبق على جهاز متوقف فقط.

## الأوامر

```bash
macxvm gpu macos                     # IOMMU/GPU في المضيف وkext الفعلي في الضيف
macxvm optimize macos --check        # هل مسار Bochs مطبق أصلًا؟
macxvm optimize macos                # جهاز متوقف، 1920x1080
macxvm optimize macos --width 1440 --height 900
```

## خلاصة صريحة

- macOS يتعرف على العرض: `AppleBochVGAFB` محمّل ويظهر في معلومات النظام.
- وهو إطار ثنائي الأبعاد بلا Metal، بحكم تصميم التعريف.
- تمرير NVIDIA ممنوع بعائقين: غياب IOMMU وغياب تعريف macOS.
- المكاسب الحقيقية: عدد الأنوية والذاكرة وأعلام تعليمات المعالج وتقليل عمل الواجهة.
