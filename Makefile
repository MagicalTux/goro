#!/bin/make
GOROOT:=$(shell PATH="/pkg/main/dev-lang.go.dev/bin:$$PATH" go env GOROOT)
GOPATH:=$(shell $(GOROOT)/bin/go env GOPATH)

all:
	GOROOT="$(GOROOT)" $(GOPATH)/bin/goimports -w -l .
	$(GOROOT)/bin/go build -v

sapi/php-cli/php-cli:
	make -C sapi/php-cli

php-cli: $(shell find -iname '*.go')
	echo building cli
	make buildext
	go build -gcflags="all=-N -l" ./sapi/php-cli

sapi/php-httpd/php-httpd:
	make -C sapi/php-httpd

sapi: sapi/php-cli/php-cli sapi/php-httpd/php-httpd

test:
	$(GOROOT)/bin/go test

http:
	$(RM) sapi/php-httpd/php-httpd
	make sapi/php-httpd/php-httpd
	./sapi/php-httpd/php-httpd

buildext:
	go run tools/buildext/buildext.go

deps:
	$(GOROOT)/bin/go get -v .

testdeps:
	$(GOROOT)/bin/go get -v -t .

travis:
	# get deps
	$(GOROOT)/bin/go get -v -t .
	$(GOROOT)/bin/go build -v
	make -C sapi/php-cgi
	make -C sapi/php-cli
	make -C sapi/php-fpm
	make -C sapi/php-httpd

.PHONY: test http buildext sapi testdeps deps travis
