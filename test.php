<?php

// this is a comment

function test($a, $b) {
	return $a + $b;
}

$test = test(4, "3.2");
echo 'This is a ' . $test . "\n";

if ($test > 5) {
	echo "Test is >5 \n";
} else {
	echo "Test is not over five\n";
}

