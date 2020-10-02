ifndef GOPATH
	GOPATH := $(HOME)/go
endif

ifndef GOOS
	GOOS := linux
endif

ifndef GO111MODULE
	GO111MODULE := on
endif

VERSION ?= "master"

.PHONY: build

all: build

build_dataflow:
	GOPATH=$(GOPATH) GOOS=$(GOOS) GO111MODULE=$(GO111MODULE) go install github.com/Donders-Institute/tg-toolset-golang/dataflow/...

build_project:
	GOPATH=$(GOPATH) GOOS=$(GOOS) GO111MODULE=$(GO111MODULE) go install github.com/Donders-Institute/tg-toolset-golang/project/...

build:
	GOPATH=$(GOPATH) GOOS=$(GOOS) GO111MODULE=$(GO111MODULE) go install github.com/Donders-Institute/tg-toolset-golang/...

build_repocli_macosx:
	GOPATH=$(GOPATH) GOOS=darwin GOARCH=amd64 GO111MODULE=$(GO111MODULE) go build -o $(GOPATH)/bin/repocli.darwin repository/cmd/repocli/main.go

build_repocli_windows:
	GOPATH=$(GOPATH) GOOS=windows GOARCH=amd64 GO111MODULE=$(GO111MODULE) go build -o $(GOPATH)/bin/repocli.exe repository/cmd/repocli/main.go

test_mailer:
	@GOPATH=$(GOPATH) GOOS=$(GOOS) GO111MODULE=$(GO111MODULE) go test -v github.com/Donders-Institute/tg-toolset-golang/pkg/mailer/...

test_store:
	@GOPATH=$(GOPATH) GOOS=$(GOOS) GO111MODULE=$(GO111MODULE) go test -v github.com/Donders-Institute/tg-toolset-golang/pkg/store/...

test_dataflow:
	@GOPATH=$(GOPATH) GOOS=$(GOOS) GO111MODULE=$(GO111MODULE) go test -v github.com/Donders-Institute/tg-toolset-golang/dataflow/...

test_project:
	@GOPATH=$(GOPATH) GOOS=$(GOOS) GO111MODULE=$(GO111MODULE) go test -v github.com/Donders-Institute/tg-toolset-golang/project/...

test_filer:
	# NOTE: the -count 1 prevents go cache used by the test causing some strange behaviour, e.g. old object UUID
	#       appears in a new object queried from the HTTP call to the filer APIs.
	@GOPATH=$(GOPATH) GOOS=$(GOOS) GO111MODULE=$(GO111MODULE) go test -count 1 -v github.com/Donders-Institute/tg-toolset-golang/project/pkg/filer/...

test_repo_db:
	@GOPATH=$(GOPATH) GOOS=$(GOOS) GO111MODULE=$(GO111MODULE) go test -v github.com/Donders-Institute/tg-toolset-golang/project/internal/cmd/repoutil/...

test:
	@GOPATH=$(GOPATH) GOOS=$(GOOS) GO111MODULE=$(GO111MODULE) go test -v github.com/Donders-Institute/tg-toolset-golang/...

release:
	VERSION=$(VERSION) rpmbuild --undefine=_disable_source_fetch -bb build/rpm/centos7.spec

github-release:
	scripts/gh-release.sh $(VERSION) false

clean:
	@rm -rf $(GOPATH)/bin/pacs_* $(GOPATH)/bin/prj_* $(GOPATH)/bin/lab_* $(GOPATH)/bin/pdb* $(GOPATH)/bin/repo*
	@rm -rf $(GOPATH)/pkg/$(GOOS)*
