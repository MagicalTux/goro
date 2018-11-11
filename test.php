<?php

function test(&$var) {
	$var = 'this is working';
}

$s = 'hello world';
var_dump($s);

test($s);

var_dump($s);
