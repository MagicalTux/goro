<?php

// this is a comment

/*
interface t {
	public static function test($a, $b);
} // */

class testclass extends \stdclass implements JsonSerializable, t {
	public static function test($a, $b) {
		return $a + $b;
	}

	public function jsonSerialize() {
	}
}

$test = testclass::test(4, "3.2");
echo 'This is a ' . $test . "\n";
var_dump($test);

var_dump(__LINE__);
