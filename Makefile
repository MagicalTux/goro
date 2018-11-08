#!/bin/make

all:
	goimports -w -l .
	go build -v

sapi/php-cli/php-cli:
	make -C sapi/php-cli

test:
	$(RM) sapi/php-cli/php-cli
	make sapi/php-cli/php-cli
	TEST_PHP_EXECUTABLE=sapi/php-cli/php-cli ./sapi/php-cli/php-cli test/php-7.2.10/run-tests.php

.PHONY: test
