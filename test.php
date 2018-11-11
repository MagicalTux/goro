<?php

$s = 'hello world';
$x = &$s;

var_dump($s, $x);
echo "\n";

$s = "nope";

var_dump($s, $x);
echo "\n";

$x = "really";

var_dump($s, $x);

