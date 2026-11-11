// MacXVM - QEMU Apple Paravirtualized Graphics Identity Probe (MIT)
// https://github.com/shadow-x78/macxvm

/*
 * The device of Apple needs the ParavirtualizedGraphics framework of the host,
 * so it exists on macOS only. This device carries the same PCI identity and
 * answers every register access with zero, and it logs each access, so a guest
 * can be measured without that framework: the log says whether macOS binds the
 * driver to the identity alone, and which registers it touches first.
 *
 * This device renders nothing. It is a probe, not an accelerator.
 *
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

#include "qemu/osdep.h"
#include "qemu/log.h"
#include "qemu/module.h"
#include "hw/pci/pci_device.h"
#include "hw/qdev-properties.h"
#include "qom/object.h"

#define TYPE_APPLE_GFX_PROBE "apple-gfx-probe"
OBJECT_DECLARE_SIMPLE_TYPE(AppleGFXProbeState, APPLE_GFX_PROBE)

#define APPLE_GFX_PROBE_VENDOR   0x106b
#define APPLE_GFX_PROBE_DEVICE   0xeeee
#define APPLE_GFX_PROBE_BAR      0
#define APPLE_GFX_PROBE_BAR_SIZE 0x1000

struct AppleGFXProbeState {
    PCIDevice parent_obj;

    MemoryRegion iomem;
    uint32_t width;
    uint32_t height;
};

static uint64_t apple_gfx_probe_read(void *opaque, hwaddr offset, unsigned size)
{
    AppleGFXProbeState *s = APPLE_GFX_PROBE(opaque);

    if (offset == 0) {
        return s->width;
    }
    if (offset == 4) {
        return s->height;
    }

    fprintf(stderr,
            "apple-gfx-probe: read  offset=0x%" HWADDR_PRIx
            " size=%u -> 0\n", offset, size);
    return 0;
}

static void apple_gfx_probe_write(void *opaque, hwaddr offset, uint64_t value,
                                  unsigned size)
{
    fprintf(stderr,
            "apple-gfx-probe: write offset=0x%" HWADDR_PRIx
            " size=%u value=0x%" PRIx64 "\n", offset, size, value);
}

static const MemoryRegionOps apple_gfx_probe_ops = {
    .read = apple_gfx_probe_read,
    .write = apple_gfx_probe_write,
    .endianness = DEVICE_LITTLE_ENDIAN,
    .valid = {
        .min_access_size = 4,
        .max_access_size = 8,
    },
    .impl = {
        .min_access_size = 4,
        .max_access_size = 4,
    },
};

static void apple_gfx_probe_realize(PCIDevice *dev, Error **errp)
{
    AppleGFXProbeState *s = APPLE_GFX_PROBE(dev);

    pci_register_bar(dev, APPLE_GFX_PROBE_BAR,
                     PCI_BASE_ADDRESS_SPACE_MEMORY, &s->iomem);

    fprintf(stderr,
            "apple-gfx-probe: the identity %04x:%04x is presented to the "
            "guest, with a %d x %d display mode\n",
            APPLE_GFX_PROBE_VENDOR, APPLE_GFX_PROBE_DEVICE,
            s->width, s->height);
}

static void apple_gfx_probe_init(Object *obj)
{
    AppleGFXProbeState *s = APPLE_GFX_PROBE(obj);

    memory_region_init_io(&s->iomem, obj, &apple_gfx_probe_ops, s,
                          TYPE_APPLE_GFX_PROBE, APPLE_GFX_PROBE_BAR_SIZE);
}

static const Property apple_gfx_probe_properties[] = {
    DEFINE_PROP_UINT32("width", AppleGFXProbeState, width, 1920),
    DEFINE_PROP_UINT32("height", AppleGFXProbeState, height, 1080),
};

static void apple_gfx_probe_class_init(ObjectClass *klass, const void *data)
{
    DeviceClass *dc = DEVICE_CLASS(klass);
    PCIDeviceClass *pci = PCI_DEVICE_CLASS(klass);

    dc->desc = "MacXVM Apple Paravirtualized Graphics identity probe";
    dc->hotpluggable = false;
    set_bit(DEVICE_CATEGORY_DISPLAY, dc->categories);

    pci->vendor_id = APPLE_GFX_PROBE_VENDOR;
    pci->device_id = APPLE_GFX_PROBE_DEVICE;
    pci->class_id = PCI_CLASS_DISPLAY_OTHER;
    pci->realize = apple_gfx_probe_realize;

    device_class_set_props(dc, apple_gfx_probe_properties);
}

static const TypeInfo apple_gfx_probe_types[] = {
    {
        .name          = TYPE_APPLE_GFX_PROBE,
        .parent        = TYPE_PCI_DEVICE,
        .instance_size = sizeof(AppleGFXProbeState),
        .class_init    = apple_gfx_probe_class_init,
        .instance_init = apple_gfx_probe_init,
        .interfaces = (const InterfaceInfo[]) {
            { INTERFACE_PCIE_DEVICE },
            { },
        },
    }
};

DEFINE_TYPES(apple_gfx_probe_types)
