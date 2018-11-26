<?php

$a = function($x) { return $x+1; };
var_dump($a);
var_dump($a(41));
exit;

$a = 1;
$b = 2;
$c = 3;
var_dump($d=$a+$b*$c);
var_dump($d);

var_dump(1+2*3);
exit;

// Provides: 2
$str = str_replace("ll", "", "good golly miss molly!", $count);
var_dump($count);


$challenge = '-aaa----b-c-----d--e---f';
echo str_replace('--', '-', $challenge)."\n";
echo preg_replace('/--+/', '-', $challenge)."\n";
