<?php

// this is a comment

function test($a, $b) {
	return $a + $b;
}

$test = test(4, "3.2");
echo 'This is a ' . $test . "\n";
var_dump($test);

$x = [1 => 2, 3 => 4, 5 => [6 => 7, 8 => 9]];
$x["hello"] = "foobar";

foreach($x as $k => $v) {
	var_dump($k, $v);
}

