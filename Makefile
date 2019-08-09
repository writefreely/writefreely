GITREV=`git describe | cut -c 2-`
LDFLAGS=-ldflags="-X 'github.com/writeas/writefreely.softwareVer=$(GITREV)'"

GOCMD=go
GOINSTALL=$(GOCMD) install $(LDFLAGS)
GOBUILD=$(GOCMD) build $(LDFLAGS)
GOTEST=$(GOCMD) test $(LDFLAGS)
GOGET=$(GOCMD) get
BINARY_NAME=writefreely
TAR_NAME=$(BINARY_NAME)_$(GITREV)
DOCKERCMD=docker
IMAGE_NAME=writeas/writefreely
TMPBIN=./tmp

all : build

ci: ci-assets deps
	cd cmd/writefreely; $(GOBUILD) -v

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

build-arm7: deps
	@hash xgo > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GOGET) -u github.com/karalabe/xgo; \
	fi
	xgo --targets=linux/arm-7, -dest build/ $(LDFLAGS) -tags='sqlite' -out writefreely ./cmd/writefreely

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
	cmd/writefreely/$(BINARY_NAME) --config
	cmd/writefreely/$(BINARY_NAME) --gen-keys
	cmd/writefreely/$(BINARY_NAME) --init-db
	cd less/; $(MAKE) install $(MFLAGS)

release : clean ui assets
	mkdir -p build/$(TAR_NAME)
	cp -r templates build/$(TAR_NAME)
	cp -r pages build/$(TAR_NAME)
	cp -r static build/$(TAR_NAME)
	mkdir build/$(TAR_NAME)/keys
	$(MAKE) build-linux
	mv build/$(BINARY_NAME)-linux-amd64 build/$(TAR_NAME)/$(BINARY_NAME)
	tar -cvzf $(TAR_NAME)_linux_amd64.tar.gz -C build $(TAR_NAME)
	rm build/$(TAR_NAME)/$(BINARY_NAME)
	$(MAKE) build-arm7
	mv build/$(BINARY_NAME)-linux-arm-7 build/$(TAR_NAME)/$(BINARY_NAME)
	tar -cvzf $(TAR_NAME)_linux_arm7.tar.gz -C build $(TAR_NAME)
	rm build/$(TAR_NAME)/$(BINARY_NAME)
	$(MAKE) build-darwin
	mv build/$(BINARY_NAME)-darwin-10.6-amd64 build/$(TAR_NAME)/$(BINARY_NAME)
	tar -cvzf $(TAR_NAME)_macos_amd64.tar.gz -C build $(TAR_NAME)
	rm build/$(TAR_NAME)/$(BINARY_NAME)
	$(MAKE) build-windows
	mv build/$(BINARY_NAME)-windows-4.0-amd64.exe build/$(TAR_NAME)/$(BINARY_NAME).exe
	cd build; zip -r ../$(TAR_NAME)_windows_amd64.zip ./$(TAR_NAME)
	rm build/$(TAR_NAME)/$(BINARY_NAME)
	$(MAKE) build-docker
	$(MAKE) release-docker

# This assumes you're on linux/amd64
release-linux : clean ui
	mkdir -p build/$(TAR_NAME)
	cp -r templates build/$(TAR_NAME)
	cp -r pages build/$(TAR_NAME)
	cp -r static build/$(TAR_NAME)
	mkdir build/$(TAR_NAME)/keys
	$(MAKE) build-no-sqlite
	mv cmd/writefreely/$(BINARY_NAME) build/$(TAR_NAME)/$(BINARY_NAME)
	tar -cvzf $(TAR_NAME)_linux_amd64.tar.gz -C build $(TAR_NAME)

release-docker :
	$(DOCKERCMD) push $(IMAGE_NAME)

ui : force_look
	cd less/; $(MAKE) $(MFLAGS)

assets : generate
	go-bindata -pkg writefreely -ignore=\\.gitignore -tags="!wflib" schema.sql sqlite.sql

assets-no-sqlite: generate
	go-bindata -pkg writefreely -ignore=\\.gitignore -tags="!wflib" schema.sql

dev-assets : generate
	go-bindata -pkg writefreely -ignore=\\.gitignore -debug -tags="!wflib" schema.sql sqlite.sql

lib-assets : generate
	go-bindata -pkg writefreely -ignore=\\.gitignore -o bindata-lib.go -tags="wflib" schema.sql

generate :
	@hash go-bindata > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GOGET) -u github.com/jteeuwen/go-bindata/go-bindata; \
	fi

$(TMPBIN):
	mkdir -p $(TMPBIN)

$(TMPBIN)/go-bindata: deps $(TMPBIN)
	$(GOBUILD) -o $(TMPBIN)/go-bindata github.com/jteeuwen/go-bindata/go-bindata

$(TMPBIN)/xgo: deps $(TMPBIN)
	$(GOBUILD) -o $(TMPBIN)/xgo github.com/karalabe/xgo

ci-assets : $(TMPBIN)/go-bindata
	$(TMPBIN)/go-bindata -pkg writefreely -ignore=\\.gitignore -tags="!wflib" schema.sql sqlite.sql

clean :
	-rm -rf build
	-rm -rf tmp
	cd less/; $(MAKE) clean $(MFLAGS)

force_look : 
	true
