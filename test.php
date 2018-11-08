<?php

// this is a comment

function test($a, $b) {
	return $a + $b;
}

$test = test(4, "3.2");
echo 'This is a ' . $test . "\n";
var_dump($test);

$x = 'hello';
var_dump($x[1]);
