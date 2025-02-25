--TEST--
Output buffering tests
--INI--
output_buffering=0
output_handler=
zlib.output_compression=0
zlib.output_handler=
--FILE--
<?php
ob_start();
echo ob_get_level();
echo 'A';
  ob_start();
  echo ob_get_level();
  echo 'B';
  $b = ob_get_contents();
  ob_end_clean();
$a = ob_get_contents();
ob_end_clean();

var_dump( $b ); // 2B
var_dump( $a ); // 1A
?>
--EXPECT--
string(2) "2B"
string(2) "1A"
