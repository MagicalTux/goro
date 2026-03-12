<?php
// Benchmark: measures interpreter performance

$start = microtime(true);

// Test 1: Variable access and arithmetic (heavy on variable lookups)
$sum = 0;
for ($i = 0; $i < 100000; $i++) {
    $sum = $sum + $i;
}
$t1 = microtime(true) - $start;
echo "Test 1 (variable + arithmetic): " . round($t1 * 1000, 2) . " ms\n";

// Test 2: Function calls (heavy on context creation)
function add($a, $b) {
    return $a + $b;
}

$start = microtime(true);
$sum = 0;
for ($i = 0; $i < 50000; $i++) {
    $sum = add($sum, $i);
}
$t2 = microtime(true) - $start;
echo "Test 2 (function calls): " . round($t2 * 1000, 2) . " ms\n";

// Test 3: Array operations
$start = microtime(true);
$arr = [];
for ($i = 0; $i < 10000; $i++) {
    $arr[] = $i;
}
$sum = 0;
foreach ($arr as $v) {
    $sum = $sum + $v;
}
$t3 = microtime(true) - $start;
echo "Test 3 (array operations): " . round($t3 * 1000, 2) . " ms\n";

// Test 4: String concatenation
$start = microtime(true);
$str = "";
for ($i = 0; $i < 10000; $i++) {
    $str = $str . "x";
}
$t4 = microtime(true) - $start;
echo "Test 4 (string concat): " . round($t4 * 1000, 2) . " ms\n";

// Test 5: Nested function calls
function fib($n) {
    if ($n <= 1) return $n;
    return fib($n - 1) + fib($n - 2);
}

$start = microtime(true);
$result = fib(20);
$t5 = microtime(true) - $start;
echo "Test 5 (recursive fib(20)): " . round($t5 * 1000, 2) . " ms\n";

$total = $t1 + $t2 + $t3 + $t4 + $t5;
echo "\nTotal: " . round($total * 1000, 2) . " ms\n";
