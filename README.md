# Goro

[![Build Status](https://travis-ci.org/MagicalTux/goro.svg)](https://travis-ci.org/MagicalTux/goro)
[![GoDoc](https://godoc.org/github.com/MagicalTux/goro/core?status.svg)](https://godoc.org/github.com/MagicalTux/goro)
[![Telegram](https://img.shields.io/badge/chat-telegram-blue.svg?logo=telegram&logoColor=white)](https://t.me/gorophp)

This is an implementation of PHP, written in pure Go (as much as possible, right now pcre doesn't exist in pure go and require usage of libpcre).

## Why?

That's a good question. PHP is a nice language but is having trouble keeping up with modern languages. This implementation, while far from finished, is trying to make a number of things possible:

* Usage of goroutines, go channels, etc from within PHP
* Better caching of compiled code by allowing sharing of compiled or live objects (classes, objects, etc) between running PHP scripts
* Use Go's memory management within PHP
* Ability to run functions or code sandboxed (including filesystem) to limit security risks
* Easily call the PHP engine from Go to execute pieces of PHP code (user provided or legacy)

# Install

In order to run php code as cli:

	go get github.com/MagicalTux/goro/sapi/php-cli

# Status

This is still a very early implementation and most of the core is missing.

## TODO

The following needs to be done before we can call this an alpha version.

* Fix arguments passed by reference for builtin methods
* Error reporting, lower level of errors (warnings, etc)
* php.ini handling
* Streams & filesystem support
* command line flags handling
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
* Allow process to run in containers on linux (see https://github.com/lizrice/containers-from-scratch - would be useful to create a play interface for goro)

## Extensions

| Extension  | Status | Notes                                          |
|------------|--------|------------------------------------------------|
| standard   |     5% | a lot of work remaining                        |
| pcre       |     5% | currently using cgo version of libpcre         |
| gmp        |    20% | Using math/big                                 |
| date       |     1% |                                                |
| hash       |    50% |                                                |
| iconv      |        | using golang.org/x/text/transform              |
| json       |        | Need custom impl to support php arrays         |
| curl       |        | using net/http                                 |
| mbstring   |        |                                                |
| mcrypt     |        |                                                |
| mysqli     |        |                                                |
| openssl    |        | using crypto packages                          |
| Reflection |        |                                                |
| session    |        |                                                |
| tokenizer  |        | already in core/tokenizer, needs to be exposed |
| xml...     |        |                                                |
| gd         |        | without gd                                     |
| bz2        |     1% |                                                |
| zlib       |        |                                                |
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

A lot of work is needed. The most important part is improving the documentation, followed by improving support of core features (see TODO in this file), and implement individual functions from the various PHP extensions.

Writing an extension is simple. Create a directory in ext, write code prefixed with a magic function defining comment and run the tools/buildext.php script to generate bindings/etc. Add your newly created extension to the main.go of each sapi to enable it by default.

