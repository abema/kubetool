# Original https://github.com/aktau/github-release/blob/master/Makefile

LAST_TAG := $(shell git describe --abbrev=0 --tags)

EXECUTABLE := kubetool

UPLOAD_CMD = github-release upload -u abema -r $(EXECUTABLE) -t $(LAST_TAG) -n $(subst /,-,$(FILE)) -f bin/$(FILE)

UNIX_EXECUTABLES := \
	darwin/amd64/$(EXECUTABLE) \
	freebsd/amd64/$(EXECUTABLE) \
	linux/amd64/$(EXECUTABLE)
WIN_EXECUTABLES := \
	windows/amd64/$(EXECUTABLE).exe

COMPRESSED_EXECUTABLES=$(UNIX_EXECUTABLES:%=%.tar.gz) $(WIN_EXECUTABLES:%.exe=%.zip)
COMPRESSED_EXECUTABLE_TARGETS=$(COMPRESSED_EXECUTABLES:%=bin/%)

prepare:
	go get github.com/aktau/github-release

all: $(EXECUTABLE)

# the executable used to perform the upload, dogfooding and all...
bin/tmp/$(EXECUTABLE):
	go build -o "$@"

# arm
bin/linux/arm/5/$(EXECUTABLE):
	GOARM=5 GOARCH=arm GOOS=linux go build -o "$@"
bin/linux/arm/7/$(EXECUTABLE):
	GOARM=7 GOARCH=arm GOOS=linux go build -o "$@"

# 386
bin/darwin/386/$(EXECUTABLE):
	GOARCH=386 GOOS=darwin go build -o "$@"
bin/linux/386/$(EXECUTABLE):
	GOARCH=386 GOOS=linux go build -o "$@"
bin/windows/386/$(EXECUTABLE):
	GOARCH=386 GOOS=windows go build -o "$@"

# amd64
bin/freebsd/amd64/$(EXECUTABLE):
	GOARCH=amd64 GOOS=freebsd go build -o "$@"
bin/darwin/amd64/$(EXECUTABLE):
	GOARCH=amd64 GOOS=darwin go build -o "$@"
bin/linux/amd64/$(EXECUTABLE):
	GOARCH=amd64 GOOS=linux go build -o "$@"
bin/windows/amd64/$(EXECUTABLE).exe:
	GOARCH=amd64 GOOS=windows go build -o "$@"

%.tar.gz: %
	tar -zcvf "$<.tar.gz" "$<"
%.zip: %.exe
	zip "$@" "$<"

# git tag -a v$(RELEASE) -m 'release $(RELEASE)'
release: bin/tmp/$(EXECUTABLE) $(COMPRESSED_EXECUTABLE_TARGETS)
	git push && git push --tags
	github-release release -u abema -r $(EXECUTABLE) \
		-t $(LAST_TAG) -n $(LAST_TAG) || true
	$(foreach FILE,$(COMPRESSED_EXECUTABLES),$(UPLOAD_CMD);)

$(EXECUTABLE):
	go build -o "$@"
