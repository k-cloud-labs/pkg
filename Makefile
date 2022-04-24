GO ?= go
GOFMT ?= gofmt "-s"
PACKAGES ?= $(shell $(GO) list ./...)
VETPACKAGES ?= $(shell $(GO) list ./... | grep -v /examples/)
GOFILES := $(shell find . -name "*.go" | grep -v vendor)
TESTFOLDER := $(shell $(GO) list ./... | grep -v examples)
TESTTAGS ?= ""

DATE := $(shell date +%Y.%m.%d-%H%M)
LATEST_COMMIT := $(shell git log --pretty=format:'%h' -n 1)
BRANCH := $(shell git branch |grep -v "no branch"| grep \*|cut -d ' ' -f2)
BUILT_ON_IP := $(shell [ $$(uname) = Linux ] && hostname -i || hostname )
RUNTIME_VER := $(shell $(GO) version)

BUILT_ON_OS := $(shell uname -a)
ifeq ($(BRANCH),)
BRANCH := master
endif

.PHONY: test
test: fmt-check vet
	echo "mode: count" > coverage.xml
	for d in $(TESTFOLDER); do \
		$(GO) test -tags $(TESTTAGS) -v -covermode=count -coverprofile=profile.out $$d > tmp.out; \
		cat tmp.out; \
		if grep -q "^--- FAIL" tmp.out; then \
			rm tmp.out; \
			exit 1; \
		elif grep -q "build failed" tmp.out; then \
			rm tmp.out; \
			exit 1; \
		elif grep -q "setup failed" tmp.out; then \
			rm tmp.out; \
			exit 1; \
		fi; \
		if [ -f profile.out ]; then \
			cat profile.out | grep -v "mode:" >> coverage.xml; \
			rm profile.out; \
		fi; \
	done

.PHONY: fmt
fmt:
	$(GOFMT) -w $(GOFILES)

.PHONY: fmt-check
fmt-check:
	@diff=$$($(GOFMT) -d $(GOFILES)); \
	if [ -n "$$diff" ]; then \
		echo "Please run 'make fmt' and commit the result:"; \
		echo "$${diff}"; \
		exit 1; \
	fi;

vet:
	$(GO) vet $(VETPACKAGES)