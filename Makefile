GITREV=`git describe | cut -c 2-`
LDFLAGS=-ldflags="-s -w -X 'github.com/writefreely/writefreely.softwareVer=$(GITREV)'"

GOCMD=go
GOINSTALL=$(GOCMD) install $(LDFLAGS)
GOBUILD=$(GOCMD) build $(LDFLAGS)
GOTEST=$(GOCMD) test $(LDFLAGS)
GOGET=$(GOCMD) get
BINARY_NAME=writefreely
BUILDPATH=build/$(BINARY_NAME)
DOCKERCMD=docker
IMAGE_NAME=writeas/writefreely
TMPBIN=./tmp

all : build

ci: deps
	cd cmd/writefreely; $(GOBUILD) -v

build: deps
	cd cmd/writefreely; $(GOBUILD) -v -tags='netgo sqlite'

build-no-sqlite: deps-no-sqlite
	cd cmd/writefreely; $(GOBUILD) -v -tags='netgo' -o $(BINARY_NAME)

build-linux: deps
	@hash xgo > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GOCMD) install src.techknowlogick.com/xgo@latest; \
	fi
	xgo --targets=linux/amd64, -dest build/ $(LDFLAGS) -tags='netgo sqlite' -go go-1.19.x -out writefreely -pkg ./cmd/writefreely .

build-windows: deps
	@hash xgo > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GOCMD) install src.techknowlogick.com/xgo@latest; \
	fi
	xgo --targets=windows/amd64, -dest build/ $(LDFLAGS) -tags='netgo sqlite' -go go-1.19.x -out writefreely -pkg ./cmd/writefreely .

build-darwin: deps
	@hash xgo > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GOCMD) install src.techknowlogick.com/xgo@latest; \
	fi
	xgo --targets=darwin/amd64, -dest build/ $(LDFLAGS) -tags='netgo sqlite' -go go-1.19.x -out writefreely -pkg ./cmd/writefreely .

build-arm6: deps
	@hash xgo > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GOCMD) install src.techknowlogick.com/xgo@latest; \
	fi
	xgo --targets=linux/arm-6, -dest build/ $(LDFLAGS) -tags='netgo sqlite' -go go-1.19.x -out writefreely -pkg ./cmd/writefreely .

build-arm7: deps
	@hash xgo > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GOCMD) install src.techknowlogick.com/xgo@latest; \
	fi
	xgo --targets=linux/arm-7, -dest build/ $(LDFLAGS) -tags='netgo sqlite' -go go-1.19.x -out writefreely -pkg ./cmd/writefreely .

build-arm64: deps
	@hash xgo > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GOCMD) install src.techknowlogick.com/xgo@latest; \
	fi
	xgo --targets=linux/arm64, -dest build/ $(LDFLAGS) -tags='netgo sqlite' -go go-1.19.x -out writefreely -pkg ./cmd/writefreely .

build-docker :
	$(DOCKERCMD) build -t $(IMAGE_NAME):latest -t $(IMAGE_NAME):$(GITREV) .

test:
	$(GOTEST) -v ./...

run:
	$(GOINSTALL) -tags='netgo sqlite' ./...
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

release : clean ui
	mkdir -p $(BUILDPATH)
	cp -r templates $(BUILDPATH)
	cp -r pages $(BUILDPATH)
	cp -r static $(BUILDPATH)
	rm -r $(BUILDPATH)/static/local
	scripts/invalidate-css.sh $(BUILDPATH)
	mkdir $(BUILDPATH)/keys
	$(MAKE) build-linux
	mv build/$(BINARY_NAME)-linux-amd64 $(BUILDPATH)/$(BINARY_NAME)
	tar -cvzf $(BINARY_NAME)_$(GITREV)_linux_amd64.tar.gz -C build $(BINARY_NAME)
	rm $(BUILDPATH)/$(BINARY_NAME)
	$(MAKE) build-arm6
	mv build/$(BINARY_NAME)-linux-arm-6 $(BUILDPATH)/$(BINARY_NAME)
	tar -cvzf $(BINARY_NAME)_$(GITREV)_linux_arm6.tar.gz -C build $(BINARY_NAME)
	rm $(BUILDPATH)/$(BINARY_NAME)
	$(MAKE) build-arm7
	mv build/$(BINARY_NAME)-linux-arm-7 $(BUILDPATH)/$(BINARY_NAME)
	tar -cvzf $(BINARY_NAME)_$(GITREV)_linux_arm7.tar.gz -C build $(BINARY_NAME)
	rm $(BUILDPATH)/$(BINARY_NAME)
	$(MAKE) build-arm64
	mv build/$(BINARY_NAME)-linux-arm64 $(BUILDPATH)/$(BINARY_NAME)
	tar -cvzf $(BINARY_NAME)_$(GITREV)_linux_arm64.tar.gz -C build $(BINARY_NAME)
	rm $(BUILDPATH)/$(BINARY_NAME)
	$(MAKE) build-darwin
	mv build/$(BINARY_NAME)-darwin-10.12-amd64 $(BUILDPATH)/$(BINARY_NAME)
	tar -cvzf $(BINARY_NAME)_$(GITREV)_macos_amd64.tar.gz -C build $(BINARY_NAME)
	rm $(BUILDPATH)/$(BINARY_NAME)
	$(MAKE) build-windows
	mv build/$(BINARY_NAME)-windows-4.0-amd64.exe $(BUILDPATH)/$(BINARY_NAME).exe
	cd build; zip -r ../$(BINARY_NAME)_$(GITREV)_windows_amd64.zip ./$(BINARY_NAME)
	rm $(BUILDPATH)/$(BINARY_NAME).exe
	$(MAKE) build-docker
	$(MAKE) release-docker

# This assumes you're on linux/amd64
release-linux : clean ui
	mkdir -p $(BUILDPATH)
	cp -r templates $(BUILDPATH)
	cp -r pages $(BUILDPATH)
	cp -r static $(BUILDPATH)
	mkdir $(BUILDPATH)/keys
	$(MAKE) build-no-sqlite
	mv cmd/writefreely/$(BINARY_NAME) $(BUILDPATH)/$(BINARY_NAME)
	tar -cvzf $(BINARY_NAME)_$(GITREV)_linux_amd64.tar.gz -C build $(BINARY_NAME)

release-docker :
	$(DOCKERCMD) push $(IMAGE_NAME)

ui : force_look
	cd less/; $(MAKE) $(MFLAGS)
	cd prose/; $(MAKE) $(MFLAGS)

$(TMPBIN):
	mkdir -p $(TMPBIN)

$(TMPBIN)/xgo: deps $(TMPBIN)
	$(GOBUILD) -o $(TMPBIN)/xgo src.techknowlogick.com/xgo

clean :
	-rm -rf build
	-rm -rf tmp
	cd less/; $(MAKE) clean $(MFLAGS)

force_look : 
	true
