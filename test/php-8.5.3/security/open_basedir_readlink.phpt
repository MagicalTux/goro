--TEST--
Test open_basedir configuration
--SKIPIF--
<?php
if(PHP_OS_FAMILY === "Windows") {
    die('skip not for Windows');
}
?>
--FILE--
<?php
chdir(__DIR__);
ini_set("open_basedir", ".");
require_once "open_basedir.inc";
$initdir = getcwd();
test_open_basedir_before("readlink", FALSE);

chdir($initdir);

$target = ($initdir."/test/bad/bad.txt");
$symlink = ($initdir."/test/ok/symlink.txt");
var_dump(symlink($target, $symlink));

chdir($initdir."/test/ok");
ini_set("open_basedir", ".");

// readlink checks input path against basedir, not the symlink target
// Paths within basedir succeed even if the symlink target is outside basedir
var_dump(readlink("symlink.txt") === $target);
var_dump(readlink("./symlink.txt") === $target);
var_dump(readlink($initdir."/test/ok/symlink.txt") === $target);

// Paths outside basedir fail
var_dump(readlink("../bad/bad.txt"));
var_dump(readlink($initdir."/test/bad/bad.txt"));

test_open_basedir_after("readlink");
?>
--CLEAN--
<?php
chdir(__DIR__);
require_once "open_basedir.inc";
delete_directories();
?>
--EXPECTF--
*** Testing open_basedir configuration [readlink] ***
bool(true)
bool(true)
bool(true)
bool(true)
bool(true)
bool(true)
bool(true)
bool(true)

Warning: readlink(): open_basedir restriction in effect. File(../bad/bad.txt) is not within the allowed path(s): (%s) in %s on line %d
bool(false)

Warning: readlink(): open_basedir restriction in effect. File(%s/test/bad/bad.txt) is not within the allowed path(s): (%s) in %s on line %d
bool(false)
*** Finished testing open_basedir configuration [readlink] ***
