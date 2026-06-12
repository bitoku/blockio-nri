PLUGIN_NAME ?= nri-iops
PLUGIN_IDX ?= 90
BINARY = $(PLUGIN_IDX)-$(PLUGIN_NAME)
INSTALL_DIR ?= /opt/nri/plugins
CONFIG_DIR ?= /etc/nri/conf.d

.PHONY: build install test clean

build:
	go build -trimpath -o $(BINARY) .

install: build
	install -d $(INSTALL_DIR)
	install -m 755 $(BINARY) $(INSTALL_DIR)/
	install -d $(CONFIG_DIR)
	install -m 644 examples/nri-iops.conf $(CONFIG_DIR)/$(BINARY).conf

test:
	go test -v ./...

clean:
	rm -f $(BINARY)
