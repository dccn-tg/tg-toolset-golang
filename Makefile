GOPATH := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
PREFIX ?= "/opt/project"

ifndef GOOS
	GOOS := linux
endif

all: build

$(GOPATH)/bin/dep:
	mkdir -p $(GOPATH)/bin
	curl https://raw.githubusercontent.com/golang/dep/master/install.sh | GOPATH=$(GOPATH) GOOS=$(GOOS) sh

build_dep: $(GOPATH)/bin/dep
	cd src/dccn.nl; GOPATH=$(GOPATH) GOOS=$(GOOS) $(GOPATH)/bin/dep ensure

update_dep: $(GOPATH)/bin/dep
	cd src/dccn.nl; GOPATH=$(GOPATH) GOOS=$(GOOS) $(GOPATH)/bin/dep ensure --update

build: build_dep
	GOPATH=$(GOPATH) GOOS=$(GOOS) go install dccn.nl/...

doc:
	@GOPATH=$(GOPATH) GOOS=$(GOOS) godoc -http=:6060

test: build_dep
	@GOPATH=$(GOPATH) GOOS=$(GOOS) GOCACHE=off go test -v dccn.nl/project/... dccn.nl/dataflow/...

install: build
	@install -D $(GOPATH)/bin/* $(PREFIX)/bin

clean:
	@rm -rf bin
	@rm -rf pkg
