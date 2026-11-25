PREFIX ?= $(HOME)/.local
SWAY_DIR ?= $(HOME)/.config/sway
WAYBAR_DIR ?= $(HOME)/.config/waybar

swaytier: *.go go.mod
	go build -trimpath -ldflags "-s -w" -o $@ .

test:
	go vet ./...
	go test ./...

install: swaytier
	install -Dm755 swaytier $(PREFIX)/bin/swaytier
	ln -sfn $(CURDIR)/sway/swaytier.conf $(SWAY_DIR)/swaytier.conf
	if [ -d $(WAYBAR_DIR) ]; then ln -sfn $(CURDIR)/waybar/swaytier.json $(CURDIR)/waybar/swaytier.css $(WAYBAR_DIR)/; fi

.PHONY: test install
