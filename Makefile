GITREV=`git describe --tags | cut -c 2-`
LDFLAGS=-ldflags="-X 'github.com/writeas/writefreely.softwareVer=$(GITREV)'"

GOCMD=go
GOINSTALL=$(GOCMD) install $(LDFLAGS)
GOBUILD=$(GOCMD) build $(LDFLAGS)
GOTEST=$(GOCMD) test $(LDFLAGS)
GOGET=$(GOCMD) get
BINARY_NAME=writefreely
DOCKERCMD=docker
IMAGE_NAME=writeas/writefreely
TMPBIN=./tmp

all : build

ci: ci-assets deps $(TMPBIN)/xgo
	$(TMPBIN)/xgo -v -tags='sqlite' ./cmd/writefreely

build: assets deps
	cd cmd/writefreely; $(GOBUILD) -v -tags='sqlite'

build-no-sqlite: assets-no-sqlite deps-no-sqlite
	cd cmd/writefreely; $(GOBUILD) -v -o $(BINARY_NAME)

build-linux: deps
	@hash xgo > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GOGET) -u github.com/karalabe/xgo; \
	fi
	xgo --targets=linux/amd64, -dest build/ $(LDFLAGS) -tags='sqlite' -out writefreely ./cmd/writefreely

build-windows: deps
	@hash xgo > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GOGET) -u github.com/karalabe/xgo; \
	fi
	xgo --targets=windows/amd64, -dest build/ $(LDFLAGS) -tags='sqlite' -out writefreely ./cmd/writefreely

build-darwin: deps
	@hash xgo > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GOGET) -u github.com/karalabe/xgo; \
	fi
	xgo --targets=darwin/amd64, -dest build/ $(LDFLAGS) -tags='sqlite' -out writefreely ./cmd/writefreely

build-docker :
	$(DOCKERCMD) build -t $(IMAGE_NAME):latest -t $(IMAGE_NAME):$(GITREV) .

test:
	$(GOTEST) -v ./...

run: dev-assets
	$(GOINSTALL) -tags='sqlite' ./...
	$(BINARY_NAME) --debug

deps :
	$(GOGET) -tags='sqlite' -d -v ./...

deps-no-sqlite:
	$(GOGET) -d -v ./...

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

# This assumes you're on linux/amd64
release-linux : clean ui
	mkdir build
	cp -r templates build
	cp -r pages build
	cp -r static build
	mkdir build/keys
	$(MAKE) build-no-sqlite
	mv cmd/writefreely/$(BINARY_NAME) build/$(BINARY_NAME)
	cd build; tar -cvzf ../$(BINARY_NAME)_$(GITREV)_linux_amd64.tar.gz *

release-docker :
	$(DOCKERCMD) push $(IMAGE_NAME)
	
ui : force_look
	cd less/; $(MAKE) $(MFLAGS)

assets : generate
	go-bindata -pkg writefreely -ignore=\\.gitignore schema.sql sqlite.sql

assets-no-sqlite: generate
	go-bindata -pkg writefreely -ignore=\\.gitignore schema.sql

dev-assets : generate
	go-bindata -pkg writefreely -ignore=\\.gitignore -debug schema.sql sqlite.sql

generate :
	@hash go-bindata > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GOGET) -u github.com/jteeuwen/go-bindata/...; \
	fi

$(TMPBIN):
	mkdir -p $(TMPBIN)

$(TMPBIN)/go-bindata: deps $(TMPBIN)
	$(GOBUILD) -o $(TMPBIN)/go-bindata github.com/jteeuwen/go-bindata/go-bindata

$(TMPBIN)/xgo: deps $(TMPBIN)
	$(GOBUILD) -o $(TMPBIN)/xgo github.com/karalabe/xgo

ci-assets : $(TMPBIN)/go-bindata
	$(TMPBIN)/go-bindata -pkg writefreely -ignore=\\.gitignore schema.sql sqlite.sql

clean :
	-rm -rf build
	-rm -rf tmp
	cd less/; $(MAKE) clean $(MFLAGS)

force_look : 
	true
