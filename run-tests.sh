#!/bin/bash

make php-cli
./test/run-tests.php -p ./php-cli $@
