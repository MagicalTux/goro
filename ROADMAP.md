# Goro Roadmap

**Current state (March 2026):** 1101 / 2000 PHP 8.5.4 tests passing (56.7%), 57 skipped, 842 failing.

This document tracks the major work areas needed to reach full PHP 8.5 compatibility, organized by impact (number of test fixes) and estimated effort.

---

## Tier 1: High Impact (50+ tests each)

### Attributes & Reflection (~110 tests failing)
Attribute parsing and storage is complete. Reflection classes exist with `getAttributes()` on all major types. Failures are likely in attribute argument evaluation, `newInstance()`, `isRepeated()`, `getTarget()`, and edge cases around namespace resolution and built-in attributes like `#[\Deprecated]`, `#[\Override]`, `#[\SensitiveParameter]`.

**Key work:**
- Audit `ReflectionAttribute::newInstance()` — must instantiate the attribute class with evaluated arguments
- Ensure `Attribute::TARGET_*` and `Attribute::IS_REPEATABLE` flags are enforced
- Validate attribute placement rules (e.g., `#[\Attribute]` on non-attribute class)
- Fix namespace/use-alias resolution in attribute class names

### Date Extension (~57 tests failing)
Core date functions (`date`, `strtotime`, `mktime`, DateTime classes) are implemented. Failures are in edge cases: unusual format strings, timezone handling, DST transitions, relative date parsing (`"next Thursday"`, `"+1 month"` on Jan 31), and `DateInterval` formatting.

**Key work:**
- Fix `strtotime()` edge cases (relative dates, timezone abbreviations)
- Complete `DateInterval::format()` with all format specifiers
- Fix DST transition handling in timezone conversions
- Implement missing `DatePeriod` iteration

### Closures (~60 tests failing)
Closure creation, binding, and invocation work. Failures are in debug output format (var_dump/array cast of closures), scope binding edge cases (`bindTo(null, null)`, static closure + `$this`), and closure name format in stack traces.

**Key work:**
- Fix closure name format: PHP 8.4+ uses `{closure:filename:line}` consistently
- Fix `(array)$closure` cast — should produce same keys as `__debugInfo`
- Fix scope preservation when rebinding with `bindTo(null)` / `bind(null)`
- Implement `#[\AllowDynamicProperties]` enforcement on Closure (prohibit `$closure->x = 1`)

---

## Tier 2: Medium Impact (20-50 tests each)

### Exceptions & Stack Traces (~49 tests failing)
Exception throwing/catching works. Failures are in stack trace format (missing constructor frame, wrong `args` format), exception context after partial construction, and error message format matching.

**Key work:**
- Add constructor frame to `getTrace()` when exception is thrown inside `__construct`
- Fix `getTraceAsString()` argument truncation format
- Ensure partially-constructed objects remain accessible after exception in constructor
- Fix error message format for TypeError/ArgumentCountError to match PHP exactly

### Constants & Const Expressions (~71 tests combined)
Basic constants work. Failures are in const expressions (operators like `>>`, `~`, complex ternary), magic constants in const context (`__LINE__`), `define()` error handling, and warning generation during const evaluation.

**Key work:**
- Support all operators in constant expressions (bitwise, shift, ternary)
- Emit warnings during const evaluation (e.g., "Trying to access array offset on null")
- Fix `define()` error messages and type validation to match PHP 8
- Fix `constant()` lookup for class constants and error handling

### Enum (~36 tests failing)
Basic enum functionality works (cases, backed enums, `from()`/`tryFrom()`). Failures are in magic method restrictions, reflection, `cases()` method, and `__CLASS__`/`__FUNCTION__` resolution inside enum methods.

**Key work:**
- Enforce magic method restrictions (prohibit `__construct`, `__destruct`, `__clone`, `__sleep`, `__wakeup`, `__set_state`, `__debugInfo`)
- Fix `__CLASS__` to resolve to enum name inside enum methods
- Complete `ReflectionEnum` and `ReflectionEnumBackedCase` / `ReflectionEnumUnitCase`
- Implement `UnitEnum::cases()` and `BackedEnum::from()`/`tryFrom()` error messages

### Exit/Die (~26 tests failing)
`exit`/`die` work as language constructs and are registered as functions. `exit(...)`/`die(...)` first-class callable syntax works. Most failures require `exec()` support (running sub-processes) or AST printing (`die` should print as `\exit()` in assertion messages).

**Key work:**
- Implement AST printing for assertions — `die` should render as `\exit()`, `exit(42)` as `\exit(42)`
- Fix exit code propagation through `exec()`/`shell_exec()`
- Support `exit` as function in `define()` context (parse error expected)

### Asymmetric Visibility (~22 tests failing)
Parsing and storage of `private(set)`/`protected(set)` is implemented. Failures are likely in enforcement: preventing writes from wrong scope, interaction with `__set`/`__get` magic, inheritance rules, and readonly + asymmetric combinations.

**Key work:**
- Enforce set-visibility on property writes from outside allowed scope
- Handle `__set` fallback when property is asymmetric and unset
- Validate inheritance: child class cannot widen set-visibility
- Fix error message format: "Cannot modify private(set) property X::$y from scope Z"

### Dynamic Calls & call_user_func (~36 tests combined)
Basic `call_user_func`/`call_user_func_array` work. Failures are in method visibility enforcement (private/protected access through callbacks), error message formatting, and `$this->$method()` dynamic dispatch.

**Key work:**
- Enforce visibility rules in `call_user_func` matching the caller's scope
- Fix error messages: "Call to private method X::y() from scope Z"
- Handle `self::`/`parent::` in callback strings with proper deprecation warnings

---

## Tier 3: Lower Impact (10-20 tests each)

### Error Messages (~16 tests)
Various error/warning messages don't match PHP's exact format. Common patterns: "Interface not found" vs "Class not found", stack trace format in fatal errors, HTML error formatting.

### Constructor Promotion (~16 tests)
Promoted properties exist but type validation on assignment may be incomplete. Fix: enforce type hints on promoted properties and match error messages.

### Class Alias (~16 tests)
`class_alias()` is implemented. Failures likely in edge cases: aliasing to existing class, aliasing interfaces/traits, and autoload interaction.

### CType Extension (~15 tests)
All 11 functions are implemented with PHP 8.1+ string-only behavior. Failures need investigation — likely edge cases with empty strings, multibyte, or locale-dependent behavior.

### Clone (~12 tests)
`clone` and `clone($obj, [...])` (PHP 8.5) are implemented. Failures likely in `__clone` method interaction, readonly property cloning, and error messages.

### Assert (~12 tests)
`assert_options()` exists but `assert()` as a language construct is not implemented. Need to add `assert()` with expression evaluation and configurable behavior (warning, exception, callback).

### Backtrace (~11 tests)
`debug_backtrace()` works but may be missing `args` key or have wrong format for some frame types.

### Class Name (~10 tests)
`::class` constant resolution may have edge cases in namespace contexts or with string expressions.

### Declare (~8 tests)
`declare(strict_types=1)` works. Failures likely in `declare(encoding=...)` validation and `declare(ticks=...)` handler.

---

## Tier 4: Individual Bug Fixes (~200 tests)

The remaining ~200 failures are individual bugs spread across many PHP features. Common themes:
- **Reference handling** — by-ref assignment to overloaded properties, reference counting edge cases
- **Array operations** — `array_splice` with iterator, `array_unshift` COW, indirect modification warnings
- **Type coercion** — compound assignment with type conversion, implicit float-to-int
- **Error handling** — specific error/warning messages not matching PHP format
- **Object operations** — dynamic property deprecation, property hooks, readonly enforcement

---

## Not Currently Tested (Beyond the 2000 test subset)

The full PHP test suite has ~18,000 tests. Additional areas not covered by our current 2000-test subset:
- **Sessions** — Not implemented
- **Streams** — Partial (file streams work, network streams not implemented)
- **XML/DOM** — Not implemented
- **GD (image processing)** — Not implemented
- **MySQLi/PDO** — Not implemented
- **OpenSSL** — Not implemented
- **Sockets** — Not implemented
- **Phar** — Not implemented
- **Intl** — Not implemented
- **Zlib/Bzip2** — Partial (decompression only)
- **CURL** — Not implemented (could map to `net/http`)

---

## Milestone Targets

| Milestone | Tests Passing | Key Deliverables |
|-----------|--------------|-------------------|
| Current | 1073 / 2000 | Core language, basic extensions |
| M1 (55%) | ~1100 | Closure debug info, exception stack traces, const expressions |
| M2 (60%) | ~1200 | Attribute reflection, enum validation, error message fixes |
| M3 (65%) | ~1300 | Date extension fixes, asymmetric visibility enforcement, assert() |
| M4 (70%) | ~1400 | Dynamic call visibility, class alias edge cases, reference handling |
| M5 (75%) | ~1500 | Exit AST printing, backtrace args, misc bug fixes |
| M6 (80%) | ~1600 | Remaining individual bug fixes |
| Full suite | ~18000 | Sessions, XML, GD, MySQLi, OpenSSL, etc. |
