#!/bin/bash

# runs the tests that are expected to pass,
# useful for quickly checking if I broke something
go build && ./run-tests.sh --show-diff \
    test/php-7.2.10/lang \
    test/php-7.2.10/basic/array \
    test/php-7.2.10/basic/strings
    test/php-7.2.10/strings
