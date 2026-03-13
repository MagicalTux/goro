--TEST--
preg_last_error() and preg_last_error_msg() basic tests
--FILE--
<?php
// After successful match
preg_match('/foo/', 'foobar');
var_dump(preg_last_error());
var_dump(preg_last_error_msg());

// Constants exist
var_dump(PREG_NO_ERROR);
var_dump(PREG_INTERNAL_ERROR);
var_dump(PREG_BACKTRACK_LIMIT_ERROR);
?>
--EXPECT--
int(0)
string(8) "No error"
int(0)
int(1)
int(2)
