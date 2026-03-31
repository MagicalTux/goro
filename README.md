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

Goro passes **~8,600 of 12,110 tests** (71.0%) from the PHP 8.5.4 test suite. First 2,000 tests: **1,752/2,000 (90.0%)**. PHP memory_limit enforcement (128MB default). Includes ext/openssl (Go crypto), getimagesize (16 formats, 35/35 pass), stream filters (73%), lazy objects (62%), PCRE2 via gopcre2, and 6 new extensions (session, xml, curl, sockets, zlib).

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
| Union/intersection types | Done |
| DNF types (`(A&B)|C`, PHP 8.2) | Partial |
| Generators (`yield`, `yield from`) | Done |
| First-class callables (`strlen(...)`) | Done |
| Property hooks (PHP 8.4) | Done |
| Asymmetric visibility (PHP 8.4) | Done |
| Lazy objects (PHP 8.4) | Partial |
| `clone ... with` (PHP 8.5) | Done |
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

| Extension | Functions | Pass Rate | Notes |
|-----------|-----------|-----------|-------|
| standard | 435+ | ~70% | Core functions, arrays, strings, files, math, output buffering, streams |
| ctype | 11 | 98% | Complete |
| json | 5 | 63% | json_encode, json_decode, json_validate, error handling |
| pcre | 11 | 67% | preg_match, preg_replace, preg_split, preg_grep — **PCRE2 via gopcre2** (backreferences, lookahead) |
| hash | 11 | 47% | hash, hash_hmac, hash_file, hash_pbkdf2, hash_hkdf, incremental |
| gmp | 45+ | 60% | Arithmetic, division, modular, bitwise, primes, GCD/LCM, factorial, operator overloading, import/export |
| mbstring | 40+ | ~50% | strlen, substr, strpos, strtolower/upper, convert_encoding, detect_encoding, check_encoding |
| date | 30+ | 62% | date, time, strtotime, mktime, DateTime, DateTimeImmutable, DateInterval, DatePeriod, DateTimeZone |
| openssl | 12 | — | AES/DES/RSA/ECDSA encryption, signing, key generation via Go crypto |
| bz2 | 2 | — | Compress (gobzip2) and decompress (Go stdlib) |
| zlib | 20+ | — | gzcompress/gzencode/gzdeflate, gzip file ops, stream filters, compress.zlib:// |
| session | 18 | — | session_start/id/destroy, file-based storage, $_SESSION superglobal |
| xml | 15+ | — | SimpleXMLElement class, xml_parser_create/parse, simplexml_load_string/file |
| curl | 12 | — | CurlHandle class, curl_init/setopt/exec/getinfo via Go net/http |
| sockets | 20+ | — | Socket class, socket_create/bind/listen/accept/connect, stream_socket_*, fsockopen |
| spl | 40+ | 82% | ArrayObject, ArrayIterator, SplFileObject, SplFixedArray, SplHeap, SplObjectStorage, iterators |
| reflection | 8 classes | 69% | ReflectionClass (with __toString), ReflectionMethod, ReflectionProperty, ReflectionFunction, ReflectionParameter, ReflectionAttribute |
| getimagesize | — | 100% | 16 image formats (JPEG, PNG, GIF, BMP, WebP, AVIF, HEIF, TIFF, PSD, etc.) |

### Not yet implemented

| Extension | Notes |
|-----------|-------|
| iconv | Planned via `golang.org/x/text/transform` |
| mysqli/PDO | Database extensions |
| gd | Image processing |
| intl | Internationalization (ICU) |
| Phar | PHP archive format |

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
