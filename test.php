<?php

// this is a comment

/*
interface t {
	public static function test($a, $b);
} // */

$php = "/usr/bin/php";
//define('TESTED_PHP_VERSION', `$php -n -r "echo PHP_VERSION;"`);

//var_dump(TESTED_PHP_VERSION);
function toto() {
	return [__DIR__, __FILE__, __LINE__];
}
var_dump(toto());

class testclass extends \stdclass implements JsonSerializable {
	var $foo;

	public static function test($a, $b) {
		return $a + $b;
	}

	public function jsonSerialize() {
	}
}

$test = testclass::test(4, "3.2");
echo 'This is a ' . $test . "\n";
var_dump($test);
var_dump((string)$test);

