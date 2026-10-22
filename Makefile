# ─────────────────────────────────────────────
# MacXVM - Build (MIT)
# https://github.com/shadow-x78/macxvm
# ─────────────────────────────────────────────

# ── Build ──
build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o bin/macxvm ./cmd/macxvm

# ── Test ──
test:
	go vet ./... && go test ./...

# ── Install ──
install: build
	mkdir -p $(HOME)/.local/bin
	install -m 0755 bin/macxvm $(HOME)/.local/bin/macxvm

# ── Clean ──
clean:
	rm -rf bin
