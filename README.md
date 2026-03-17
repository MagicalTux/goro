# Goro

[![CI](https://github.com/MagicalTux/goro/actions/workflows/ci.yml/badge.svg)](https://github.com/MagicalTux/goro/actions/workflows/ci.yml)
[![Coverage Status](https://coveralls.io/repos/github/MagicalTux/goro/badge.svg?branch=master)](https://coveralls.io/github/MagicalTux/goro?branch=master)
[![GoDoc](https://godoc.org/github.com/MagicalTux/goro/core?status.svg)](https://godoc.org/github.com/MagicalTux/goro)

PHP engine implemented in pure Go. Targets latest PHP (currently 8.5) compatibility.

## Why?

PHP is a nice language but is having trouble keeping up with modern languages. This implementation makes a number of things possible:

* Usage of goroutines, go channels, etc from within PHP
* Better caching of compiled code by allowing sharing of compiled or live objects (classes, objects, etc) between running PHP scripts
* Use Go's memory management within PHP
* Ability to run functions or code sandboxed (including filesystem via `fs.FS`) to limit security risks
* Easily call the PHP engine from Go to execute pieces of PHP code (user provided or legacy)

## Install

```bash
go install github.com/MagicalTux/goro/sapi/php-cli@latest
```

## Status

Goro passes **1301 tests** from the PHP 8.5.4 test suite (1301 pass, 57 skipped for platform-specific reasons).

### Language Features

| Feature | Status |
|---------|--------|
| Variables, constants, type juggling | Done |
| Control flow (if/else/switch/match/for/foreach/while) | Done |
| Functions, closures, arrow functions (`fn`) | Done |
| Classes, interfaces, traits, abstract classes | Done |
| Enums (PHP 8.1) | Done |
| Namespaces | Done |
| Exceptions (try/catch/finally/throw) | Done |
| Error hierarchy (Error, TypeError, ValueError, etc) | Done |
| Null coalescing (`??`, `??=`) | Done |
| Spaceship operator (`<=>`) | Done |
| Spread operator (`...`) | Done |
| Named arguments (PHP 8.0) | Done |
| Match expression (PHP 8.0) | Done |
| Nullsafe operator (`?->`, PHP 8.0) | Done |
| Readonly properties (PHP 8.1) | Done |
| Fibers (PHP 8.1) | Done |
| Union/intersection types | Partial |
| Generators (`yield`) | Done |
| First-class callables (`strlen(...)`) | Done |
| Attributes | Parsed, partially enforced |

### SAPIs

| SAPI | Status |
|------|--------|
| CLI (`php-cli`) | Working |
| CGI (`php-cgi`) | Working |
| FPM (`php-fpm`) | Working |
| HTTP handler (`php-httpd`) | Working |
| Test runner (`php-test`) | Working |

### Extensions

| Extension | Functions | Status | Notes |
|-----------|-----------|--------|-------|
| standard | 435+ | ~70% | Core functions, arrays, strings, files, math, output buffering, streams |
| ctype | 11 | 100% | Complete |
| json | 5 | 100% | json_encode, json_decode, json_validate, error handling |
| pcre | 11 | 90% | preg_match, preg_replace, preg_split, preg_grep, preg_filter, callbacks |
| hash | 11 | 80% | hash, hash_hmac, hash_file, hash_hmac_file, hash_pbkdf2, hash_hkdf, incremental |
| gmp | 31 | 45% | Arithmetic, division, modular, bitwise, primes, GCD/LCM, factorial |
| mbstring | 22 | 55% | strlen, substr, strpos, convert_encoding, convert_case, ord/chr, list_encodings |
| date | 13 | 30% | date, time, strtotime, mktime, strftime, getdate, checkdate, DateTime class |
| bz2 | 1 | 10% | Decompression only (Go stdlib lacks bzip2 writer) |
| spl | 6+ classes | 15% | Countable, OuterIterator, ArrayIterator, InfiniteIterator, SplFixedArray, SplDoublyLinkedList |
| reflection | 5 classes | 15% | ReflectionClass, ReflectionMethod, ReflectionProperty, ReflectionFunction, ReflectionParameter |
| pcre | | | Uses Go's `regexp` (RE2 syntax, not PCRE2) |

### Not yet implemented

| Extension | Notes |
|-----------|-------|
| session | |
| iconv | Planned via `golang.org/x/text/transform` |
| curl | Planned via `net/http` |
| mysqli | |
| openssl | Planned via `crypto` packages |
| xml / DOM | |
| gd | |
| zlib | |
| Phar | |
| sockets | |

## Architecture

### Process

A process object is typically created once per runtime environment. It caches compiled code and holds global stream wrapper resources, persistent connections, and runtime cache.

### Global

When a request is received or script execution is requested, a new Global context is created. It contains runtime state: global variables, declared functions, classes, constants, output buffers, and memory limits.

### Context

Context is a local scope (e.g., within a running function). Global has a root context, and each function call creates a new context to separate variable scope.

## Contributing

See [development.md](development.md) for details on writing extensions.

Writing an extension: create a directory in `ext/`, write functions with magic comment prefixes, run `make buildext` to generate bindings, and add the extension import to each SAPI's `main.go`.
