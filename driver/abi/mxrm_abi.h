// MacXVM - MXGPU ABI (MIT)
// https://github.com/shadow-x78/macxvm

#ifndef MXGPU_MXRM_ABI_H
#define MXGPU_MXRM_ABI_H

#include <stdint.h>

#define MXRM_ABI_VERSION_MAJOR 1
#define MXRM_ABI_VERSION_MINOR 0
#define MXRM_ABI_VERSION_PATCH 0

#define MXRM_ABI_VERSION ((MXRM_ABI_VERSION_MAJOR << 16) | \
                          (MXRM_ABI_VERSION_MINOR << 8) | \
                          (MXRM_ABI_VERSION_PATCH))

#define MXRM_MAX_BARS 6
#define MXRM_MAX_HEADS 4
#define MXRM_MAX_CHANNELS 16
#define MXRM_MAX_CLIENTS 8
#define MXRM_MAX_NAME 64

typedef struct mxrm_bar_info {
    uint64_t start;
    uint64_t end;
    uint64_t size;
    uint32_t flags;
    uint32_t index;
} mxrm_bar_info;

typedef struct mxrm_identity {
    uint32_t abi_version;
    uint32_t client_index;
    uint64_t client_id;
    char name[MXRM_MAX_NAME];
    uint16_t vendor_id;
    uint16_t device_id;
    uint16_t subsystem_vendor_id;
    uint16_t subsystem_device_id;
    uint8_t revision;
    uint8_t head_count;
    uint16_t channel_count;
    uint64_t framebuffer_size;
    uint64_t vram_size;
    mxrm_bar_info bars[MXRM_MAX_BARS];
    uint32_t bar_count;
} mxrm_identity;

typedef enum {
    MXRM_STATE_RESET = 0,
    MXRM_STATE_PROBED = 1,
    MXRM_STATE_STARTED = 2,
    MXRM_STATE_SUSPENDED = 3,
    MXRM_STATE_STOPPED = 4,
    MXRM_STATE_FAILED = 5
} mxrm_state;

typedef enum {
    MXRM_FLAG_NONE = 0,
    MXRM_FLAG_REBAR = 1 << 0,
    MXRM_FLAG_COHERENT = 1 << 1,
    MXRM_FLAG_RESIZABLE_BAR = 1 << 2
} mxrm_flags;

typedef struct mxrm_start_request {
    uint32_t abi_version;
    uint32_t flags;
    uint64_t client_id;
    uint32_t head_count;
    uint32_t channel_count;
    uint64_t framebuffer_size;
} mxrm_start_request;

typedef struct mxrm_status {
    uint32_t abi_version;
    uint32_t state;
    uint32_t flags;
    uint32_t error_code;
    char message[MXRM_MAX_NAME];
} mxrm_status;

int mxrm_probe(mxrm_identity *identity);
int mxrm_start(const mxrm_start_request *request, mxrm_status *status);
int mxrm_stop(mxrm_status *status);
int mxrm_suspend(mxrm_status *status);
int mxrm_resume(mxrm_status *status);

#endif
