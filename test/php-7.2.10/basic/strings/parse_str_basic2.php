<?php
/* Prototype  : void parse_str  ( string $str  [, array &$arr  ] )
 * Description: Parses the string into variables
 * Source code: ext/standard/string.c
*/

echo "*** Testing parse_str() : non-default arg_separator.input specified ***\n";

$s1 = "first=val1/second=val2/third=val3";
var_dump(parse_str($s1, $result));
var_dump($result);

?>
===DONE===
