# make(1) from NetBSD (bmake on many Linux's) uses $(.CURDIR), but GNU Make uses
# $(CURDIR). Normalize on $(.CURDIR) by setting it to $(CURDIR) if not set, and
# then $(PWD) if still not set.
.CURDIR ?= $(CURDIR)
.CURDIR ?= $(PWD)

# For reproducible builds, don't store the current build directory by default;
# instead store only the path from the root of the repo (so there is still
# enough info in debug messages to find the correct file).
GCFLAGS  = -gcflags="all=-trimpath=$(.CURDIR)"
ASMFLAGS = -asmflags="all=-trimpath=$(.CURDIR)"

GITREV!=git describe --tags | cut -c 2-
GOLDFLAGS=-ldflags="-X 'github.com/writeas/writefreely.softwareVer=$(GITREV)' -extldflags '$(LDFLAGS)'"

GOCMD=go
GOINSTALL=$(GOCMD) install $(GOLDFLAGS) $(GCFLAGS) $(ASMFLAGS)
GOBUILD=$(GOCMD) build $(GOLDFLAGS) $(GCFLAGS) $(ASMFLAGS)
GOTEST=$(GOCMD) test $(GOLDFLAGS) $(GCFLAGS) $(ASMFLAGS)
GOGET=$(GOCMD) get
BINARY_NAME=writefreely
DOCKERCMD=docker
IMAGE_NAME=writeas/writefreely

all : build

build: assets deps
	cd cmd/writefreely; $(GOBUILD) -v -tags='sqlite'

build-linux: deps
	@hash xgo > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GOGET) -u github.com/karalabe/xgo; \
	fi
	xgo --targets=linux/amd64, -dest build/ $(GOLDFLAGS) $(GCFLAGS) $(ASMFLAGS) -tags='sqlite' -out writefreely ./cmd/writefreely

build-windows: deps
	@hash xgo > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GOGET) -u github.com/karalabe/xgo; \
	fi
	xgo --targets=windows/amd64, -dest build/ $(GOLDFLAGS) $(GCFLAGS) $(ASMFLAGS) -tags='sqlite' -out writefreely ./cmd/writefreely

build-darwin: deps
	@hash xgo > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GOGET) -u github.com/karalabe/xgo; \
	fi
	xgo --targets=darwin/amd64, -dest build/ $(GOLDFLAGS) $(GCFLAGS) $(ASMFLAGS) -tags='sqlite' -out writefreely ./cmd/writefreely

build-docker :
	$(DOCKERCMD) build -t $(IMAGE_NAME):latest -t $(IMAGE_NAME):$(GITREV) .

test:
	$(GOTEST) -v ./...

run: dev-assets
	$(GOINSTALL) -tags='sqlite' ./...
	$(BINARY_NAME) --debug

deps :
	$(GOGET) -tags='sqlite' -v ./...

install : build
	cmd/writefreely/$(BINARY_NAME) --gen-keys
	cd less/; $(MAKE) install $(MFLAGS)

release : clean ui assets
	mkdir build
	cp -r templates build
	cp -r pages build
	cp -r static build
	mkdir build/keys
	$(MAKE) build-linux
	mv build/$(BINARY_NAME)-linux-amd64 build/$(BINARY_NAME)
	cd build; tar -cvzf ../$(BINARY_NAME)_$(GITREV)_linux_amd64.tar.gz *
	rm build/$(BINARY_NAME)
	$(MAKE) build-darwin
	mv build/$(BINARY_NAME)-darwin-10.6-amd64 build/$(BINARY_NAME)
	cd build; tar -cvzf ../$(BINARY_NAME)_$(GITREV)_macos_amd64.tar.gz *
	rm build/$(BINARY_NAME)
	$(MAKE) build-windows
	mv build/$(BINARY_NAME)-windows-4.0-amd64.exe build/$(BINARY_NAME).exe
	cd build; zip -r ../$(BINARY_NAME)_$(GITREV)_windows_amd64.zip ./*
	$(MAKE) build-docker
	$(MAKE) release-docker

release-docker :
	$(DOCKERCMD) push $(IMAGE_NAME)
	
ui : force_look
	cd less/; $(MAKE) $(MFLAGS)

assets : generate
	go-bindata -pkg writefreely -ignore=\\.gitignore schema.sql sqlite.sql

dev-assets : generate
	go-bindata -pkg writefreely -ignore=\\.gitignore -debug schema.sql sqlite.sql

generate :
	@hash go-bindata > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GOGET) -u github.com/jteeuwen/go-bindata/...; \
	fi

clean :
	-rm -rf build
	cd less/; $(MAKE) clean $(MFLAGS)

force_look : 
	true
