GITREV=`git describe | cut -c 2-`
LDFLAGS=-ldflags="-s -w -X 'github.com/writefreely/writefreely.softwareVer=$(GITREV)' -extldflags '-static'"
BASELDFLAGS=-ldflags="-s -w -X 'github.com/writefreely/writefreely.softwareVer=$(GITREV)'"

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

# Build tags used for normal (disk-based) builds. static/, templates/, and
# pages/ -- except file-based overrides like static/local/custom.css -- are
# read from disk at runtime, so `make ui` / editing those dirs is picked up
# without a rebuild.
TAGS=netgo sqlite
NOSQLITE_TAGS=netgo

# Adding the `embed` tag compiles the contents of static/, templates/, and
# pages/ into the binary. File-based overrides are still read from disk and
# take precedence over their embedded counterparts. This is what release
# builds use, so they produce a single self-contained binary.
EMBED_TAGS=$(TAGS) embed

all : build

ci: deps
	cd cmd/writefreely; $(GOBUILD) -v

build: deps
	cd cmd/writefreely; $(GOBUILD) -v -tags='$(TAGS)'

# build-embed produces a binary with static/templates/pages embedded, for
# locally testing embedded release builds without running the full `release`
# target.
build-embed: deps
	cd cmd/writefreely; $(GOBUILD) -v -tags='$(EMBED_TAGS)'

build-no-sqlite: deps-no-sqlite
	cd cmd/writefreely; $(GOBUILD) -v -tags='$(NOSQLITE_TAGS)' -o $(BINARY_NAME)

build-linux: deps
	@hash xgo > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GOCMD) install src.techknowlogick.com/xgo@latest; \
	fi
	xgo --targets=linux/amd64, -dest build/ $(LDFLAGS) -tags='$(TAGS)' -go go-1.25.x -out writefreely -pkg ./cmd/writefreely .

build-windows: deps
	@hash xgo > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GOCMD) install src.techknowlogick.com/xgo@latest; \
	fi
	xgo --targets=windows/amd64, -dest build/ $(LDFLAGS) -tags='$(TAGS)' -go go-1.25.x -out writefreely -pkg ./cmd/writefreely .

build-darwin: deps
	@hash xgo > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GOCMD) install src.techknowlogick.com/xgo@latest; \
	fi
	xgo --targets=darwin/amd64, -dest build/ $(BASELDFLAGS) -tags='$(TAGS)' -go go-1.25.x -out writefreely -pkg ./cmd/writefreely .

build-darwin-arm64: deps
	@hash xgo > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GOCMD) install src.techknowlogick.com/xgo@latest; \
	fi
	xgo --targets=darwin/arm64, -dest build/ $(BASELDFLAGS) -tags='$(TAGS)' -go go-1.25.x -out writefreely -pkg ./cmd/writefreely .

build-arm6: deps
	@hash xgo > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GOCMD) install src.techknowlogick.com/xgo@latest; \
	fi
	xgo --targets=linux/arm-6, -dest build/ $(LDFLAGS) -tags='$(TAGS)' -go go-1.25.x -out writefreely -pkg ./cmd/writefreely .

build-arm7: deps
	@hash xgo > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GOCMD) install src.techknowlogick.com/xgo@latest; \
	fi
	xgo --targets=linux/arm-7, -dest build/ $(LDFLAGS) -tags='$(TAGS)' -go go-1.25.x -out writefreely -pkg ./cmd/writefreely .

build-arm64: deps
	@hash xgo > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GOCMD) install src.techknowlogick.com/xgo@latest; \
	fi
	xgo --targets=linux/arm64, -dest build/ $(LDFLAGS) -tags='$(TAGS)' -go go-1.25.x -out writefreely -pkg ./cmd/writefreely .

build-docker :
	$(DOCKERCMD) build -t $(IMAGE_NAME):latest -t $(IMAGE_NAME):$(GITREV) .

test:
	$(GOTEST) -v ./...

run:
	$(GOINSTALL) -tags='$(TAGS)' ./...
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

# static/, templates/, and pages/ are compiled into each release binary via
# the `embed` build tag (see EMBED_TAGS above), so none of them are shipped
# as loose files alongside the binary anymore. File-based overrides like
# static/local/custom.css still work: an admin can create the relevant
# directory next to the binary and it takes precedence over the embedded
# copy.
#
# Because `embed` reads templates/ from source rather than from $(BUILDPATH),
# the CSS cache-busting rewrite has to run against the tracked templates/
# directory itself instead of a disposable copy. It's reverted via
# `git checkout` once every platform binary has been built.
release : clean ui
	mkdir -p $(BUILDPATH)
	mkdir $(BUILDPATH)/keys
	scripts/invalidate-css.sh .
	$(MAKE) build-linux TAGS='$(EMBED_TAGS)'
	mv build/$(BINARY_NAME)-linux-amd64 $(BUILDPATH)/$(BINARY_NAME)
	tar -cvzf $(BINARY_NAME)_$(GITREV)_linux_amd64.tar.gz -C build $(BINARY_NAME)
	rm $(BUILDPATH)/$(BINARY_NAME)
	$(MAKE) build-arm6 TAGS='$(EMBED_TAGS)'
	mv build/$(BINARY_NAME)-linux-arm-6 $(BUILDPATH)/$(BINARY_NAME)
	tar -cvzf $(BINARY_NAME)_$(GITREV)_linux_arm6.tar.gz -C build $(BINARY_NAME)
	rm $(BUILDPATH)/$(BINARY_NAME)
	$(MAKE) build-arm7 TAGS='$(EMBED_TAGS)'
	mv build/$(BINARY_NAME)-linux-arm-7 $(BUILDPATH)/$(BINARY_NAME)
	tar -cvzf $(BINARY_NAME)_$(GITREV)_linux_arm7.tar.gz -C build $(BINARY_NAME)
	rm $(BUILDPATH)/$(BINARY_NAME)
	$(MAKE) build-arm64 TAGS='$(EMBED_TAGS)'
	mv build/$(BINARY_NAME)-linux-arm64 $(BUILDPATH)/$(BINARY_NAME)
	tar -cvzf $(BINARY_NAME)_$(GITREV)_linux_arm64.tar.gz -C build $(BINARY_NAME)
	rm $(BUILDPATH)/$(BINARY_NAME)
	$(MAKE) build-darwin TAGS='$(EMBED_TAGS)'
	mv build/$(BINARY_NAME)-darwin-10.12-amd64 $(BUILDPATH)/$(BINARY_NAME)
	tar -cvzf $(BINARY_NAME)_$(GITREV)_macos_amd64.tar.gz -C build $(BINARY_NAME)
	rm $(BUILDPATH)/$(BINARY_NAME)
	$(MAKE) build-darwin-arm64 TAGS='$(EMBED_TAGS)'
	mv build/$(BINARY_NAME)-darwin-10.12-arm64 $(BUILDPATH)/$(BINARY_NAME)
	tar -cvzf $(BINARY_NAME)_$(GITREV)_macos_arm64.tar.gz -C build $(BINARY_NAME)
	rm $(BUILDPATH)/$(BINARY_NAME)
	$(MAKE) build-windows TAGS='$(EMBED_TAGS)'
	mv build/$(BINARY_NAME)-windows-4.0-amd64.exe $(BUILDPATH)/$(BINARY_NAME).exe
	cd build; zip -r ../$(BINARY_NAME)_$(GITREV)_windows_amd64.zip ./$(BINARY_NAME)
	rm $(BUILDPATH)/$(BINARY_NAME).exe
	git checkout -- templates

# This assumes you're on linux/amd64
release-linux : clean ui
	mkdir -p $(BUILDPATH)
	mkdir $(BUILDPATH)/keys
	$(MAKE) build-no-sqlite NOSQLITE_TAGS='netgo embed'
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
