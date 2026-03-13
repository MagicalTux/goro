--TEST--
array_any(), array_all(), array_find(), array_find_key() basic tests
--FILE--
<?php
// array_any
var_dump(array_any([1, 2, 3], fn($v) => $v > 2)); // true
var_dump(array_any([1, 2, 3], fn($v) => $v > 5)); // false
var_dump(array_any([], fn($v) => true)); // false (empty)

// array_all
var_dump(array_all([1, 2, 3], fn($v) => $v > 0)); // true
var_dump(array_all([1, 2, 3], fn($v) => $v > 2)); // false
var_dump(array_all([], fn($v) => false)); // true (empty)

// array_find
var_dump(array_find([1, 2, 3], fn($v) => $v > 2)); // int(3)
var_dump(array_find([1, 2, 3], fn($v) => $v > 5)); // NULL
var_dump(array_find(['a' => 1, 'b' => 2, 'c' => 3], fn($v, $k) => $k === 'b')); // int(2)

// array_find_key
var_dump(array_find_key([1, 2, 3], fn($v) => $v > 2)); // int(2)
var_dump(array_find_key([1, 2, 3], fn($v) => $v > 5)); // NULL
var_dump(array_find_key(['a' => 1, 'b' => 2, 'c' => 3], fn($v, $k) => $k === 'b')); // string(1) "b"
?>
--EXPECT--
bool(true)
bool(false)
bool(false)
bool(true)
bool(false)
bool(true)
int(3)
NULL
int(2)
int(2)
NULL
string(1) "b"
