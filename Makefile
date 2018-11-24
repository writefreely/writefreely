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
	cd cmd/writefreely; GOOS=linux GOARCH=amd64 $(GOBUILD) -v

build-windows: deps
	cd cmd/writefreely; GOOS=windows GOARCH=amd64 $(GOBUILD) -v

build-darwin: deps
	cd cmd/writefreely; GOOS=darwin GOARCH=amd64 $(GOBUILD) -v

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
	$(MAKE) build-linux
	cp cmd/writefreely/$(BINARY_NAME) build
	cd build; tar -cvzf ../$(BINARY_NAME)_$(GITREV)_linux_amd64.tar.gz *
	rm build/$(BINARY_NAME)
	$(MAKE) build-darwin
	cp cmd/writefreely/$(BINARY_NAME) build
	cd build; tar -cvzf ../$(BINARY_NAME)_$(GITREV)_darwin_amd64.tar.gz *
	rm build/$(BINARY_NAME)
	$(MAKE) build-windows
	cp cmd/writefreely/$(BINARY_NAME).exe build
	cd build; zip -r ../$(BINARY_NAME)_$(GITREV)_windows_amd64.zip ./*; cd ..
	$(DOCKERCMD) build -t $(IMAGE_NAME):latest -t $(IMAGE_NAME):$(GITREV) .
	
ui : force_look
	cd less/; $(MAKE) $(MFLAGS)

clean :
	-rm -rf build
	cd less/; $(MAKE) clean $(MFLAGS)

force_look : 
	true
