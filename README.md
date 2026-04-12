# Goro

[![CI](https://github.com/MagicalTux/goro/actions/workflows/ci.yml/badge.svg)](https://github.com/MagicalTux/goro/actions/workflows/ci.yml)
[![Coverage Status](https://coveralls.io/repos/github/MagicalTux/goro/badge.svg?branch=master)](https://coveralls.io/github/MagicalTux/goro?branch=master)
[![GoDoc](https://godoc.org/github.com/MagicalTux/goro/core?status.svg)](https://godoc.org/github.com/MagicalTux/goro)

PHP engine implemented in pure Go. Feature-complete for PHP 8.5 language features.

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

Goro passes **~11,823 of 12,110 tests** (~97.6%) from the PHP 8.5.4 test suite (~201 failures, 86 skipped in CI). PHP memory_limit enforcement (128MB default). Includes PCRE2 via gopcre2, IANA timezones via gotz, and 9 extensions (session, xml, curl, sockets, zlib, mysqli, sqlite3, bz2).

### Remaining test failures by area

| Area | Failures | Notes |
|------|----------|-------|
| ext/date | 51 | DatePeriod serialization format, date_parse edge cases, DST fallback transitions |
| attributes | 20 | Reflection __toString formatting, delayed target validation, AST printing |
| exceptions | 9 | __toString error location, variance autoload, stream wrappers |
| closures | 9 | Closure const expressions, binding edge cases |
| clone | 6 | AST printing, clone-with edge cases |
| exit | 5 | `exit()` in custom SAPIs, disabling exit |
| constexpr | 5 | Constant expression edge cases (new in defaults, array unpack) |
| asymmetric_visibility | 5 | Static props, nested variations, indirect modification |
| assert | 5 | assert() callback exceptions, `??=` in assert, AST pretty-printer |
| ext/mbstring | 4 | Encoding conversion edge cases |
| constants | 4 | Constant evaluation edge cases |
| ext/hash | 3 | PHP serialization format edge cases |
| ext/gmp | 3 | GMP unserialize with references |
| Other | ~75 | Scattered across ~40 areas (≤3 failures each): reference tracking, object ID ordering, warning ordering, etc. |

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
| standard | 527+ | ~70% | Core functions, arrays, strings, files, math, output buffering, streams |
| ctype | 11 | 100% | Complete |
| json | 5 | 98% | json_encode, json_decode, json_validate, error handling |
| pcre | 11 | 67% | preg_match, preg_replace, preg_split, preg_grep — **PCRE2 via gopcre2** (backreferences, lookahead) |
| hash | 14 | 94% | hash, hash_hmac, hash_file, hash_pbkdf2, hash_hkdf, incremental |
| gmp | 49 | 96% | Arithmetic, division, modular, bitwise, primes, GCD/LCM, factorial, operator overloading, import/export |
| mbstring | 49 | 97% | strlen, substr, strpos, strtolower/upper, convert_encoding, detect_encoding, check_encoding |
| date | 48 | 89% | date, time, strtotime, mktime, DateTime, DateTimeImmutable, DateInterval, DatePeriod, DateTimeZone, sunrise/sunset |
| openssl | 16 | — | AES/DES/RSA/ECDSA encryption, signing, key generation via Go crypto |
| bz2 | 2 | — | Compress (gobzip2) and decompress (Go stdlib) |
| zlib | 22 | — | gzcompress/gzencode/gzdeflate, gzip file ops, stream filters, compress.zlib:// |
| session | 19 | — | session_start/id/destroy, file-based storage, $_SESSION superglobal |
| xml | 18 | — | SimpleXMLElement class, xml_parser_create/parse, simplexml_load_string/file |
| curl | 13 | — | CurlHandle class, curl_init/setopt/exec/getinfo via Go net/http |
| sockets | 25 | — | Socket class, socket_create/bind/listen/accept/connect, stream_socket_*, fsockopen |
| mysqli | 25 | — | mysqli/mysqli_result/mysqli_stmt classes, prepared statements, transactions via go-sql-driver/mysql |
| sqlite3 | 20+ | — | SQLite3/SQLite3Result/SQLite3Stmt classes, prepared statements via glebarez/go-sqlite (pure Go) |
| spl | 40+ | 82% | ArrayObject, ArrayIterator, SplFileObject, SplFixedArray, SplHeap, SplObjectStorage, iterators |
| reflection | 8 classes | 75% | ReflectionClass (with __toString), ReflectionMethod, ReflectionProperty, ReflectionFunction, ReflectionParameter, ReflectionAttribute |
| getimagesize | — | 100% | 16 image formats (JPEG, PNG, GIF, BMP, WebP, AVIF, HEIF, TIFF, PSD, etc.) |

### Not yet implemented

| Extension | Notes |
|-----------|-------|
| PDO | Planned via database/sql (MySQL + SQLite drivers already available) |
| iconv | Planned via `golang.org/x/text/transform` |
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
