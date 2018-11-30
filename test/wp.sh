#!/bin/sh
set -e

# This will get a wordpress & run a php server using Goro in order to allow running tests
if [ -d "test" ]; then
	cd test
fi

if [ ! -d "wordpress" ]; then
	if [ ! -f wordpress.tar.gz ]; then
		echo "Downloading wordpress ..."
		wget -q -O wordpress.tar.gz https://wordpress.org/latest.tar.gz
	fi
	echo "Extracting wordpress ..."
	# assuming we are running linux
	tar xf wordpress.tar.gz
	# leaving latest.tar.gz there because it won't hurt should we need to reset this wordpress
fi

cd ..

rm -f sapi/php-httpd/php-httpd
make sapi/php-httpd/php-httpd

exec ./sapi/php-httpd/php-httpd test/wordpress
