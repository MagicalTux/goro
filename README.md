# Goro

This is an implementation of PHP, written in pure Go (as much as possible, right now pcre doesn't exist in pure go and require usage of libpcre).

## Why?

That's a good question. PHP is a nice language but is having trouble keeping up with modern languages. This implementation, while far from finished, as purpose to allow a few things from within PHP:

* Usage of goroutines, go channels, etc
* Better caching of compiled code
* Use Go's memory management within PHP
* Ability to run functions or code sandboxed (including filesystem)

# Install

In order to run php code as cli:

	go get github.com/MagicalTux/goro/sapi/php-cli

# Status

This is still a very early implementation and most of the core is missing.

Right now, running PHP's own tests would require pcre, so it is not possible to run the tests yet.

## TODO

* Fix arguments passed by reference for builtin methods
* Error reporting, lower level of errors (warnings, etc)
* php.ini handling
* Class and object implementation is still very early. Access checks are missing. Class inheritance, etc are missing
 * Class abstract
 * Builtin class methods
 * Interfaces
 * Exception throwing
 * Traits
* break/continue needs improvement/fixing
* declare
* goto ?
* namespaces
* generators
* predefined superglobals
* implement Complex (curly) variable syntax (see: http://php.net/manual/en/language.types.string.php#language.types.string.parsing.complex )

## Extensions

| Extension  | Status | Notes                                          |
|------------|--------|------------------------------------------------|
| standard   |     5% | a lot of work remaining                        |
| pcre       |     5% | might need to use cgo                          |
| gmp        |    20% | Using math/big                                 |
| date       |        |                                                |
| hash       |    50% |                                                |
| iconv      |        | using golang.org/x/text/transform              |
| json       |        | Need custom impl to support php arrays         |
| mbstring   |        |                                                |
| mcrypt     |        |                                                |
| mysqli     |        |                                                |
| openssl    |        | using crypto packages                          |
| Reflection |        |                                                |
| session    |        |                                                |
| tokenizer  |        | already in core/tokenizer, needs to be exposed |
| xml*       |        |                                                |
| gd         |        | without gd                                     |
| ctype      |   100% |                                                |

# Concepts

## Process

A process object will typically be created only once in a runtime environment,
and is used to keep some objects cached, as well as global stream wrapper
resources. Persistant connections, run time cache, etc are stored at the
Process level.

## GlobalCache (optional)

A special global status intermediate between Process and Global that causes
declarations of classes and functions to be kept between scripts, typically
useful when using an autoloader.

## Global

When a request is received or execution of a script is requested, a new Global
context is created. The global context contains runtime level details such as
global variables, declared functions, classes, constants, etc.

## Context

Context is a local context, such as within a running function etc. Global has
a root context, and each function call causes instantiation of a new context
so that variables space is kept separated.

# Contributing

Right now, code for extensions is the most needed part.

Writing an extension is simple. Create a directory in ext, write code prefixed with a magic function defining comment and run the tools/buildext.php script to generate bindings/etc. Add your newly created extension to the main.go of each sapi to enable it by default.

