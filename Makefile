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

all : build

build: deps
	cd cmd/writefreely; $(GOBUILD) -v

build-linux: deps
	@hash xgo > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GOGET) -u github.com/karalabe/xgo; \
	fi
	xgo --targets=linux/amd64, -dest build/ $(LDFLAGS) -out writefreely ./cmd/writefreely

build-windows: deps
	@hash xgo > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GOGET) -u github.com/karalabe/xgo; \
	fi
	xgo --targets=windows/amd64, -dest build/ $(LDFLAGS) -out writefreely ./cmd/writefreely

build-darwin: deps
	@hash xgo > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GOGET) -u github.com/karalabe/xgo; \
	fi
	xgo --targets=darwin/amd64, -dest build/ $(LDFLAGS) -out writefreely ./cmd/writefreely

build-docker :
	$(DOCKERCMD) build -t $(IMAGE_NAME):latest -t $(IMAGE_NAME):$(GITREV) .

test:
	$(GOTEST) -v ./...

run:
	$(GOINSTALL) ./...
	$(BINARY_NAME) --debug

deps :
	$(GOGET) -v ./...

install : build
	cmd/writefreely/$(BINARY_NAME) --gen-keys
	cd less/; $(MAKE) install $(MFLAGS)

release : clean ui
	mkdir build
	cp -r templates build
	cp -r pages build
	cp -r static build
	mkdir build/keys
	cp schema.sql build
	cp sqlite.sql build
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

clean :
	-rm -rf build
	cd less/; $(MAKE) clean $(MFLAGS)

force_look : 
	true
