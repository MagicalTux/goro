# Goro Roadmap

**Current state (April 2026):** 11,838 / 12,121 PHP 8.5.5 tests passing
(97.7%). 196 failures, 87 skipped in CI. All 20 vendored extensions
implemented in pure Go; `CGO_ENABLED=0 go build ./...` succeeds.

With the high-level features in place, remaining work is mostly a long
tail of targeted fixes: error-message format parity, a handful of
reflection/attribute edge cases, DST/DatePeriod subtleties, and a few
tests gated on language features we haven't implemented (`goto`, full
`exec()` subprocess support).

---

## Remaining failures by area

From the last CI run (commit `852b33af`). The Other category is the
long tail — ~40 distinct areas, at most three failures each.

| Area | Failures | Notes |
|------|----------|-------|
| ext/date | 51 | DatePeriod serialization format, date_parse edge cases, DST fallback transitions |
| attributes | 20 | Reflection __toString formatting, delayed target validation, AST printing |
| exceptions | 9 | __toString error location, variance autoload, stream wrappers |
| closures | 9 | Closure const expressions, binding edge cases |
| clone | 6 | AST printing, clone-with edge cases |
| exit | 5 | Tests rely on `exec()` subprocess spawning, disabling exit |
| constexpr | 5 | New-in-defaults, array unpack in const, stack trace format |
| asymmetric_visibility | 5 | Static props, nested variations, indirect modification |
| assert | 5 | Callback exceptions, `??=` in assert, AST pretty-printer |
| ext/mbstring | 4 | Encoding conversion edge cases |
| constants | 4 | Constant evaluation edge cases |
| ext/hash | 3 | PHP serialization format edge cases |
| ext/gmp | 3 | GMP unserialize with references |
| **Other** | ~67 | Scattered across ~40 areas |

### Fundamental limitations (would need engine-level work)

- **Object ID recycling** — Go's tracing GC doesn't deterministically
  free objects the way PHP's refcount-based engine does, so some tests
  that assume a specific object-id sequence drift. (~30 tests.)
- **Destructor timing** — related; deterministic destructors at scope
  exit require refcount tracking. (~20 tests.)
- **`goto`** — not implemented. (~5 tests.)
- **PCRE2 / GMP RNGs** — we use different PRNGs than PHP, so tests
  asserting exact random output under a fixed seed don't match.
- **`SensitiveParameter` backtrace redaction** — not implemented.

---

## Test suite coverage

The vendored PHP 8.5.5 test set is 12,121 tests. Ten extensions have
their tests imported (`ctype, date, gmp, hash, json, mbstring, pcre,
reflection, spl, standard`), alongside `Zend/tests` (core language)
and `tests/` (general).

| Area | Tests | Pass rate |
|------|-------|-----------|
| ext/standard | 3,808 | high |
| ext/spl | 781 | ~82% |
| ext/date | 688 | ~93% |
| ext/reflection | 493 | ~75% |
| ext/mbstring | 416 | ~97% |
| ext/pcre | 163 | 67% (PCRE2 via gopcre2) |
| ext/gmp | 99 | 97% |
| ext/json | 88 | 98% |
| ext/hash | 80 | 94% |
| ext/ctype | 49 | 100% |

Not yet vendored: `ext/{bz2, curl, gd, mbstring (some subdirs),
mysqli, openssl, session, sockets, sqlite3, xml, zlib}` tests. The
extensions themselves are implemented (see README), but the upstream
test suites aren't imported into the repo yet.

---

## Planned work

### Short-term (individual test fixes)

- **ext/date, attributes, exceptions, closures** — these four buckets
  account for ~50% of remaining failures. Each is a grab-bag of small
  fixes (error-message format, reflection output, AST printing).
- **AST pretty-printer** — multiple test areas (`assert`, `clone`,
  `attributes`, `constexpr`) depend on it matching PHP's exact format.
  A focused pass on the printer would clear a dozen tests.
- **Reflection `__toString`** — the remaining attribute/reflection
  failures largely come from slight format differences.

### Medium-term (features)

- **PDO** — via `database/sql`; the MySQL and SQLite drivers are
  already in the tree (`go-sql-driver/mysql`, `glebarez/go-sqlite`).
- **`iconv`** — maps cleanly onto `golang.org/x/text/transform`.
- **`exec()` subprocess support** — would unlock the `exit/` tests.

### Longer-term / won't-do

- **intl** — full ICU is substantial; leave unless a concrete user
  needs it.
- **Phar** — PHP archive format; rarely needed outside legacy code.
- **Deterministic object-id / destructor semantics** — would require
  moving off Go's GC onto refcounting throughout the value system.
  Test impact is ~50 scattered failures; not worth the cost.

---

## Recent milestones

- **April 2026** — PHP test suite refreshed to 8.5.5 (12,110 → 12,121
  tests). All cgo removed; pure-Go bcrypt/SHA-crypt/MD5-crypt and a
  POSIX "C" locale emulation replaced the libc wrappers.
- **March 2026** — 20th extension (gd, pure Go via gogd) added; pass
  rate crossed 97% on the full PHP suite.
- **Earlier 2026** — New extensions (session, xml, curl, sockets,
  zlib, mysqli, sqlite3, bz2); library integrations (gopcre2, gotz,
  gobzip2, go-sql-driver/mysql, glebarez/go-sqlite).
