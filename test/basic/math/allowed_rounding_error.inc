<?php
function allowed_rounding_error($actual, $expected) {
	$tolerance = 0.00001;
	return (abs($actual - $expected) < $tolerance);
}
?>
