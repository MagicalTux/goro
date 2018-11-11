<?php

var_dump( phpversion());

$s = 'hello world';

$x = &$s;

var_dump($s, $x);
echo "\n";

$x = "nope";

var_dump($s, $x);
echo "\n";

$s = "really";

var_dump($s, $x);

