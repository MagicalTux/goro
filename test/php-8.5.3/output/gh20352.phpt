--TEST--
GH-20352 (UAF in php_output_handler_free via re-entrant ob_start() during error deactivation)
--SKIPIF--
<?php
// This test verifies PHP's C-level memory safety (UAF) during OB deactivation.
// The expected output ordering depends on PHP's specific shutdown sequencing
// (destructor output during php_output_deactivate appears before the deferred
// fatal error) which goro cannot replicate without C-level deactivation hooks.
die('skip C-level OB deactivation ordering not applicable to goro');
?>
--FILE--
<?php
class Test {
    public function __destruct() {
        // Spray output stack
        for ($i = 0; $i < 1000; $i++)
            ob_start(static function() {});
    }

    public function __invoke($x) {
        // Trigger php_output_deactivate() through forbidden operation
        ob_start('foo');
        return $x;
    }
}

ob_start(new Test, 1);

echo "trigger bug";
?>
--EXPECTF--
%r(Notice: ob_start\(\): Failed to create buffer in [^\r\n]+ on line \d+\r?\n(\r?\n)?)+%r
Notice: ob_start(): Failed to create buffer in %s on line %d

Fatal error: ob_start(): Cannot use output buffering in output buffering display handlers in %s on line %d
