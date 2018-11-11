# PHP in Go

This is an implementation of PHP, written in pure Go.

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
