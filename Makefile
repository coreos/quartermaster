#
# Based on http://chrismckenzie.io/post/deploying-with-golang/
#

.PHONY: version all run dist clean

APP_NAME := quartermaster
SHA := $(shell git rev-parse --short HEAD)
BRANCH := $(subst /,-,$(shell git rev-parse --abbrev-ref HEAD))
VER := $(shell git describe)
ARCH := $(shell go env GOARCH)
GOOS := $(shell go env GOOS)
DIR=.

ifdef APP_SUFFIX
  VERSION = $(VER)-$(subst /,-,$(APP_SUFFIX))
else
ifeq (master,$(BRANCH))
  VERSION = $(VER)
else
  VERSION = $(VER)-$(BRANCH)
endif
endif

# Go setup
GO=go

# Sources and Targets
EXECUTABLES :=$(APP_NAME)
# Build Binaries setting main.version and main.build vars
LDFLAGS :=-ldflags "-X main.QUARTERMASTER_VERSION=$(VERSION) -extldflags '-z relro -z now'"
# Package target
PACKAGE :=$(DIR)/dist/$(APP_NAME)-$(VERSION).$(GOOS).$(ARCH).tar.gz
GOFILES=$(shell go list ./... | grep -v vendor)

.DEFAULT: all

all: quartermaster

glide-up:
	glide up -v

rebuild:
	glide rebuild

# print the version
version:
	@echo $(VERSION)

# print the name of the app
name:
	@echo $(APP_NAME)

# print the package path
package:
	@echo $(PACKAGE)

quartermaster:
	go build $(LDFLAGS) -o $(APP_NAME) cmd/quartermaster/main.go

run: quartermaster
	./$(APP_NAME)

test:
	go test $(GOFILES)

clean:
	@echo Cleaning Workspace...
	rm -rf $(APP_NAME)
	rm -rf dist

$(PACKAGE): all
	@echo Packaging Binaries...
	@mkdir -p tmp/$(APP_NAME)
	@cp $(APP_NAME) tmp/$(APP_NAME)/
	@mkdir -p $(DIR)/dist/
	tar -czf $@ -C tmp $(APP_NAME);
	@rm -rf tmp
	@echo
	@echo Package $@ saved in dist directory

dist: $(PACKAGE) $(CLIENT_PACKAGE)

.PHONY: server client test clean name run version
