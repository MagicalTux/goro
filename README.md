# PHP in Go

This is an implementation of PHP, written in pure Go.

## Why?

That's a good question. PHP is a nice language but is having trouble keeping up with modern languages. This implementation, while far from finished, as purpose to allow a few things from within PHP:

* Usage of goroutines, go channels, etc
* Better caching of compiled code
* Use Go's memory management within PHP
* Ability to run functions or code sandboxed (including filesystem)

# Install

In order to run php code as cli:

	go get github.com/MagicalTux/gophp/sapi/php-cli

# Status

This is still a very early implementation and most of the core is missing.

## Syntax

* Class and object implementation is still very early. Access checks are missing. Class inheritance, etc are missing
* Traits are missing
* Operator precedence is yet to be implemented (see http://php.net/manual/en/language.operators.precedence.php )
* do-while structure
* break/continue needs improvement/fixing
* declare still missing
* include/require missing
* goto missing
* namespaces are missing too
* Exception throwing missing
* generators are missing
* predefined superglobals are missing...

## Extensions

### Standard

Partial implementation.

# Contributing

Right now, code for extensions is the most needed part.

Writing an extension is simple. Create a directory in ext, write code prefixed with a magic function defining comment and run the tools/buildext.php script to generate bindings/etc. Add your newly created extension to the main.go of each sapi to enable it by default.

