PREFIX ?= $(HOME)/.local
SWAY_DIR ?= $(HOME)/.config/sway

swaytier: *.go go.mod
	go build -trimpath -ldflags "-s -w" -o $@ .

test:
	go vet ./...
	go test ./...

install: swaytier
	install -Dm755 swaytier $(PREFIX)/bin/swaytier
	ln -sfn $(CURDIR)/sway/swaytier.conf $(SWAY_DIR)/swaytier.conf

.PHONY: test install
