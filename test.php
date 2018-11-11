<?php

class foo {
	var $x = 'hi';

	public function bar($a) {
		var_dump($this);
		return $a + 1;
	}
}

$foo = new foo();
var_dump($foo->bar(41));
var_dump($foo->x);

