default: build
.PHONY: build test testacc docs local

PROJ := "godaddy-dns"
ORG := "veksh"

BINARY := "terraform-provider-$(PROJ)"
# BINARY := "terraform-provider-godaddy-dns_v$(VERSION)"
VERSION := $(shell git describe --tags --always)

ARCH := $(shell go env GOARCH)
OS := $(shell go env GOOS)

LOCAL_PATH := "~/.terraform.d/plugins/registry.terraform.io/$(ORG)/$(PROJ)/$(VERSION)/$(OS)_$(ARCH)/"

export

## cmds

build:
	go build -o bin/$(BINARY) -ldflags='-s -w -X main.version=$(VERSION)' .

test:
	go test -v -timeout=30s -parallel=4 ./...

testacc:
	TF_ACC=1 go test -v -timeout 2m ./...

docs:
	cd tools; go generate ./...; cd ..

local: build
	go build -o $(BINARY) -ldflags='-s -w -X main.version=$(VERSION)' .
	rm -rf       $(LOCAL_PATH)
	mkdir -p     $(LOCAL_PATH)
	mv $(BINARY) $(LOCAL_PATH)
	chmod +x     $(LOCAL_PATH)/$(BINARY)
