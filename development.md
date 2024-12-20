## Prerequisites
Install go-pcre dependencies:
In ubuntu:
```
sudo apt install libpcre3-dev pkg-config
```
In debian:
```
sudo apt-get install libpcre++-dev
```
For other systems, look up what the name of the pcre library is.
Also, make sure gcc is installed.

## Building and Running
To build everything:
```
make build
```

Since there's still a lot of things missing, you probably just want
to run something. To create a runnable PHP CLI binary:
```
make cli
```
A file php-cli will be created in the current directory.
To run a PHP script:
```
./php-cli test.php
```

## Extensions
The extensions are found in the ext/ directory.
The standard extension is PHP's standard library.
and the example extension is contains the code
that explains and demostrate on how
extensions work.

### Adding a new extension
To create a new extension, first create a directory in
the ext/ with the extension name:
```
mkdir ext/my_ext
```
In that directory, create a go file with any filename (except ext.go):
```
touch ext/my_ext/funcs.go
```

Then open and edit the `ext/my_ext/funcs.go` file to add the following:
```
package my_ext

import (
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)


// > func string hello ( string $name )
func fncHello(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var name phpv.ZString
	_, err := core.Expand(ctx, args, &name)
	if err != nil {
		return nil, err
	}

	result := phpv.ZStr("hello " + string(name))
	return result.ZVal(), nil
}
```
In this file, a PHP function hello will be added.
Save the file, then run
```
make buildext
```
... to generate the extension init code (TODO: this should probably done with go generate instead).

Finally, to make the extension available to the CLI, in the file sapi/php-cli/main.go,
add the following inside the import list:
```
    // ... other imports here
	_ "github.com/MagicalTux/goro/ext/my_ext"
```

Run `make cli` to make sure there are no errors.
If everything goes fine, then the function `hello` will be available for use.
Edit the test.php file:
```
<?php
echo hello("deer") . "\n";
```
Then do `./php-cli test.php` to see the output `hello deer`.


## Testing
To make sure goro is compliant and compatible with the official PHP implementation,
the official PHP tests and PHP test runner will be used. As of this writing,
goro aims to be compatible with PHP-7.3, so the php-7 tests will be used,
and can be found here https://github.com/php/php-src/tree/php-7.3.28/tests.
Each extensions also has their own comprehensive tests, such as
https://github.com/php/php-src/tree/php-7.3.28/ext/standard/tests.

But the for test runner, the latest test runner will be used:
https://github.com/php/php-src/blob/master/run-tests.php
This test runner still usable for the php-7 tests, but
more improvements and features (such as colored output).

To use the latest test runner, a php-8 must first be installed.
Then to run the basic tests, do the following:
```
./test/run-tests.php -p ./php-cli test/basic
```
The -p option allows us to use the goro PHP against the tests. Without it,
it will use the official PHP, which is not what you want for testing purposes.
The `test/basic` contains the all the basic tests. You can pick any other
directories that contain tests.

Use -h to show what options run-tests.php has:
```
./test/run-tests.php -h
```


