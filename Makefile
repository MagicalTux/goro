#!/bin/make

all:
	goimports -w -l .
	go build -v

sapi/php-cli/php-cli:
	make -C sapi/php-cli

sapi/php-httpd/php-httpd:
	make -C sapi/php-httpd

test:
	$(RM) sapi/php-cli/php-cli
	make sapi/php-cli/php-cli
	TEST_PHP_EXECUTABLE=sapi/php-cli/php-cli ./sapi/php-cli/php-cli test/php-7.2.10/run-tests.php

http:
	$(RM) sapi/php-httpd/php-httpd
	make sapi/php-httpd/php-httpd
	./sapi/php-httpd/php-httpd

buildext:
	$(RM) sapi/php-cli/php-cli
	make sapi/php-cli/php-cli
	./sapi/php-cli/php-cli tools/buildext.php

deps:
	go get -v -t .

.PHONY: test http buildext deps
