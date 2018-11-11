<?php

$array = array(
	'this',
	'is',
	'for',
	'nobody',
);

foreach($array as $v) {
	var_dump($v);
	if ($v == 'for') break;
}

for($i = 0; $i < 5; ++$i) {
	var_dump($i);
}

var_dump(++$x);
var_dump(++$x);
var_dump(++$x);
