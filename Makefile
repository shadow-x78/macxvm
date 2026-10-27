# ─────────────────────────────────────────────
# MacXVM - Build (MIT)
# https://github.com/shadow-x78/macxvm
# ─────────────────────────────────────────────

build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w -X github.com/shadow-x78/macxvm/internal/cmds.Version=0.9.0" -o bin/macxvm ./cmd/macxvm

test:
	go vet ./... && go test ./...

install: build
	mkdir -p $(HOME)/.local/bin
	install -m 0755 bin/macxvm $(HOME)/.local/bin/macxvm

integration:
	MACXVM_INTEGRATION=1 ./scripts/integration.sh $(BASE)

clean:
	rm -rf bin
