<?php

// Provides: 2
$str = str_replace("ll", "", "good golly miss molly!", $count);
var_dump($count);


$challenge = '-aaa----b-c-----d--e---f';
echo str_replace('--', '-', $challenge)."\n";
echo preg_replace('/--+/', '-', $challenge)."\n";
