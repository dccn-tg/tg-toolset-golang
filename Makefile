ifndef GOPATH
	GOPATH := $(HOME)/go
endif

ifndef GOOS
	GOOS := linux
endif

ifndef GO111MODULE
	GO111MODULE := on
endif

all: build

build_dataflow:
	GOPATH=$(GOPATH) GOOS=$(GOOS) GO111MODULE=$(GO111MODULE) go install github.com/Donders-Institute/tg-toolset-golang/dataflow/...

build_project:
	GOPATH=$(GOPATH) GOOS=$(GOOS) GO111MODULE=$(GO111MODULE) go install github.com/Donders-Institute/tg-toolset-golang/project/...

build:
	GOPATH=$(GOPATH) GOOS=$(GOOS) GO111MODULE=$(GO111MODULE) go install github.com/Donders-Institute/tg-toolset-golang/...

doc:
	@GOPATH=$(GOPATH) GOOS=$(GOOS) GO111MODULE=$(GO111MODULE) godoc -http=:6060

test_dataflow:
	@GOPATH=$(GOPATH) GOOS=$(GOOS) GO111MODULE=$(GO111MODULE) go test -v github.com/Donders-Institute/tg-toolset-golang/dataflow/...

test_project:
	@GOPATH=$(GOPATH) GOOS=$(GOOS) GO111MODULE=$(GO111MODULE) go test -v github.com/Donders-Institute/tg-toolset-golang/project/...

test:
	@GOPATH=$(GOPATH) GOOS=$(GOOS) GO111MODULE=$(GO111MODULE) go test -v github.com/Donders-Institute/tg-toolset-golang/...

clean:
	@rm -rf $(GOPATH)/bin/pacs_* $(GOPATH)/bin/prj_* $(GOPATH)/bin/lab_* $(GOPATH)/bin/pdb_*
	@rm -rf $(GOPATH)/pkg/$(GOOS)*