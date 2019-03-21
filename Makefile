ifndef GOOS
	GOOS := linux
endif

all: build

build_dataflow:
	GOPATH=$(GOPATH) GOOS=$(GOOS) go install github.com/Donders-Institute/tg-toolset-golang/dataflow/...

build_projectdb:
	GOPATH=$(GOPATH) GOOS=$(GOOS) go install github.com/Donders-Institute/tg-toolset-golang/projectdb/...

build:
	GOPATH=$(GOPATH) GOOS=$(GOOS) go install github.com/Donders-Institute/tg-toolset-golang/...

doc:
	@GOPATH=$(GOPATH) GOOS=$(GOOS) godoc -http=:6060

test_dataflow:
	@GOPATH=$(GOPATH) GOOS=$(GOOS) go test -v github.com/Donders-Institute/tg-toolset-golang/dataflow/...

test_projectdb:
	@GOPATH=$(GOPATH) GOOS=$(GOOS) go test -v github.com/Donders-Institute/tg-toolset-golang/projectdb/...

test:
	@GOPATH=$(GOPATH) GOOS=$(GOOS) go test -v github.com/Donders-Institute/tg-toolset-golang/...

clean:
	@rm -rf $(GOPATH)/bin/pacs_* $(GOPATH)/bin/prj_* $(GOPATH)/bin/lab_* $(GOPATH)/bin/pdb_*
	@rm -rf $(GOPATH)/pkg
