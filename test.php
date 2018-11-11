<?php

class foo {
	function bar($a) {
		return $a + 1;
	}
}

var_dump(foo::bar(41));
