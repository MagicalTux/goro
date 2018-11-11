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
