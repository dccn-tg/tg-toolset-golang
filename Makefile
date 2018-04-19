GOPATH := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
PREFIX ?= "/opt/cluster"

all: build

external:
	@GOPATH=$(GOPATH) GOOS=linux go get ./...

build: external
	@GOPATH=$(GOPATH) GOOS=linux go install ./...

doc:
	@GOPATH=$(GOPATH) GOOS=linux godoc -http=:6060

test: external
	@GOPATH=$(GOPATH) GOOS=linux GOCACHE=off go test -v dccn.nl/project/...

clean:
	@rm -rf bin
	@rm -rf pkg
