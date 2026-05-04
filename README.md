# Goro

[![CI](https://github.com/KarpelesLab/goro/actions/workflows/ci.yml/badge.svg)](https://github.com/KarpelesLab/goro/actions/workflows/ci.yml)
[![Coverage Status](https://coveralls.io/repos/github/KarpelesLab/goro/badge.svg?branch=master)](https://coveralls.io/github/KarpelesLab/goro?branch=master)
[![Go Reference](https://pkg.go.dev/badge/github.com/KarpelesLab/goro.svg)](https://pkg.go.dev/github.com/KarpelesLab/goro)

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
go install github.com/KarpelesLab/goro/sapi/php-cli@latest
```

## Status

Goro passes **~11,864 of 12,121 tests** (~97.9%) from the PHP 8.5.5 test suite (~170 failures, 87 skipped in CI). PHP memory_limit enforcement (128MB default). Includes PCRE2 via gopcre2, IANA timezones via gotz, and 10 extensions (session, xml, curl, gd, sockets, zlib, mysqli, sqlite3, bz2).

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
| gd | 60+ | — | GdImage class, imagecreate/truecolor, drawing (lines, rectangles, ellipses, polygons, arcs, fill), text (TTF, built-in fonts), PNG/JPEG/GIF/BMP I/O, copy/resize/resample/rotate/crop/flip/scale, filters, convolution — **pure Go via gogd** |
| getimagesize | — | 100% | 16 image formats (JPEG, PNG, GIF, BMP, WebP, AVIF, HEIF, TIFF, PSD, etc.) |

### Not yet implemented

| Extension | Notes |
|-----------|-------|
| PDO | Planned via database/sql (MySQL + SQLite drivers already available) |
| iconv | Planned via `golang.org/x/text/transform` |
| intl | Internationalization (ICU) |
| Phar | PHP archive format |

## Bytecode VM (experimental)

A stack-based bytecode VM runs in parallel to the AST tree-walking
executor. It's opt-in and falls back to the AST per-function on any
unsupported construct.

Enable it with the `GORO_VM=1` environment variable:

```bash
GORO_VM=1 php-cli script.php
```

The VM emitter currently handles:

- Scalar literals (`int`, `float`, `string`, `bool`, `null`) — including
  the case-insensitive constants `true`/`false`/`null`.
- Variable read/write, with a per-frame slot cache so reads skip the
  FuncContext hashtable entirely.
- Arithmetic / bitwise / shift / comparison / concat / unary
  (`-`, `~`, `!`); plain and compound assignment (`=`, `+=`, …);
  pre/post `++`/`--`.
- Short-circuit `&&` / `||` and `??` (null coalesce, simple-variable
  LHS only).
- `if` / `elseif` / `else`, `while`, `for`, `foreach` (value form,
  array + object iteration), `break` / `continue` (single level),
  `return`, `throw`.
- Array literals (`[…]`, including keyed `k => v`); `$a[$k]` read;
  `$a[$k] = v` and `$a[] = v` writes (with auto-vivification from
  null/false and string-offset semantics).
- Object instantiation (`new Cls(args…)`), property read
  (`$obj->prop`), method call (`$obj->m(args…)`) with full PHP
  visibility checks (private/protected) and `__call` fallback.
- Builtin and user-defined function calls (positional args).

Out of scope (falls back to AST per-function via ErrUnsupported):

- By-ref returns and by-ref parameters (the VM passes pre-evaluated
  ZVals; the AST passes Runnables and binds `Writable`s).
- `try` / `catch` / `finally` (throw is supported, exceptions
  propagate to AST callers/wrappers).
- Generators (`yield`).
- `$obj->prop = v` (deferred until a public WriteValue helper exists).
- Static-property and class-constant access (`Foo::$bar`, `Foo::CONST`).
- Nullsafe chains (`$obj?->...`).
- Spread (`...$arr`) and named arguments.
- Dynamic names (`$$x`, `$obj->{$x}`, `$f()` with $f a variable,
  `new $cls()`).
- Multi-level `break N` / `continue N`.
- Type-hinted return values (the AST coerces; the VM doesn't yet).
- User-defined constants (PHP_INT_MAX, MYAPP_FOO, …).
- List destructure, anonymous classes, `extract`/`compact`/`$$x` and
  similar locals-introspecting builtins (those force the slot cache
  to fall back; currently we just compile-time bail).

Functions matching any of the above run as AST as before — the engine
silently picks the right backend per-function, with no behaviour
change.

Bench wins (vs. AST baseline, per-iter):

| Benchmark | AST | VM | Δ |
|---|---|---|---|
| Arithmetic | 61M ns | 37M ns | **-40%** |
| Fibonacci | 25M ns | 20M ns | -18% |
| ArrayOps | 11.3M ns | 10.2M ns | -10% |
| StringConcat | 11.5M ns | 11.1M ns | -3% |
| FunctionCalls | 19.4M ns | 20.3M ns | +5% |

Larger gains require either an unboxed value type, slot-only writes
(skipping the hashtable mirror) for slot-safe functions, or
register-based opcodes. The 64-bit instruction format already has room
for the last one.

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
