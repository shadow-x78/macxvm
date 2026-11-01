# ─────────────────────────────────────────────
# MacXVM - MXGPU Driver Build (MIT)
# https://github.com/shadow-x78/macxvm
# ─────────────────────────────────────────────

SHELL := /bin/bash
ROOT := $(abspath $(dir $(lastword $(MAKEFILE_LIST)))/..)
BUILD := $(ROOT)/driver/build
DIST := $(ROOT)/driver/dist
ARTIFACTS := $(ROOT)/artifacts/mxgpu/package
KEY_ID ?= mxgpu-release-2026

.PHONY: all build test test-host package sbom clean

all: test-host

build:
	@$(ROOT)/driver/installer/build.sh

test:
	@$(ROOT)/driver/installer/build.sh --test

test-host:
	@cd $(ROOT) && go test ./internal/driver/... && $(ROOT)/scripts/check-headers.sh

package:
	@$(ROOT)/driver/installer/package.sh $(KEY_ID)

sbom:
	@$(ROOT)/driver/installer/build.sh --sbom

clean:
	rm -rf $(BUILD) $(DIST)
