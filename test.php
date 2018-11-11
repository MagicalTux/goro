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

switch($i) {
	case 4:
		echo "nope\n";
	case 5;
		echo "yes\n";
	case 6;
		echo "still yes\n";
		break;
	default:
		echo "NOOOO\n";
}

