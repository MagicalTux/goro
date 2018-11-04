<?php

$s = token_get_all(file_get_contents('test.php'));

foreach($s as $parse) {
	if (is_array($parse)) {
		echo $parse[2].': '.token_name($parse[0]).' '.json_encode($parse[1])."\n";
	} else {
		echo json_encode($parse)."\n";
	}
}
