# ─────────────────────────────────────────────
# MacXVM - Build (MIT)
# https://github.com/shadow-x78/macxvm
# ─────────────────────────────────────────────

# ── Binary ──

build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w -X github.com/shadow-x78/macxvm/internal/cmds.Version=0.10.5" -o bin/macxvm ./cmd/macxvm

install: build
	mkdir -p $(HOME)/.local/bin
	install -m 0755 bin/macxvm $(HOME)/.local/bin/macxvm

# ── Tests ──

test:
	go vet ./... && go test ./...

test-host:
	go test ./internal/driver/... && ./scripts/check-headers.sh

integration:
	MACXVM_INTEGRATION=1 ./scripts/integration.sh $(BASE)

# ── Driver ──

driver-package:
	$(MAKE) -C driver package

# ── Housekeeping ──

clean:
	rm -rf bin
