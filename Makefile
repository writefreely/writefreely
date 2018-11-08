GOCMD=go
GOINSTALL=$(GOCMD) install
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
BINARY_NAME=writefreely

all : build

build:
	cd cmd/writefreely; $(GOBUILD)

test:
	$(GOTEST) -v ./...

run:
	$(GOINSTALL) ./...
	$(BINARY_NAME) --debug

install : 
	./keys.sh
	cd less/; $(MAKE) install $(MFLAGS)

ui : force_look
	cd less/; $(MAKE) $(MFLAGS)

clean :
	cd less/; $(MAKE) clean $(MFLAGS)
	
force_look : 
	true
