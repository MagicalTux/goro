<?php

$test = 'This is a test';

var_dump(preg_replace('/This ([^ ]+) /', 'That really \\1 not much $1 of ', $test));

exit;

// Provides: <body text='black'>
$bodytag = str_replace("%body%", "black", "<body text='%body%'>");
var_dump($bodytag);

// Provides: Hll Wrld f PHP
$vowels = array("a", "e", "i", "o", "u", "A", "E", "I", "O", "U");
$onlyconsonants = str_replace($vowels, "", "Hello World of PHP");
var_dump($onlyconsonants);

// Provides: You should eat pizza, beer, and ice cream every day
$phrase  = "You should eat fruits, vegetables, and fiber every day.";
$healthy = array("fruits", "vegetables", "fiber");
$yummy   = array("pizza", "beer", "ice cream");

$newphrase = str_replace($healthy, $yummy, $phrase);
var_dump($newphrase);

// Provides: 2
$str = str_replace("ll", "", "good golly miss molly!", $count);
echo $count."\n";


// Outputs F because A is replaced with B, then B is replaced with C, and so on...
// Finally E is replaced with F, because of left to right replacements.
$search  = array('A', 'B', 'C', 'D', 'E');
$replace = array('B', 'C', 'D', 'E', 'F');
$subject = 'A';
echo str_replace($search, $replace, $subject)."\n";



// Outputs: apearpearle pear
// For the same reason mentioned above
$letters = array('a', 'p');
$fruit   = array('apple', 'pear');
$text    = 'a p';
$output  = str_replace($letters, $fruit, $text);
echo $output."\n";



$challenge = '-aaa----b-c-----d--e---f';
echo str_replace('--', '-', $challenge)."\n";
echo preg_replace('/--+/', '-', $challenge)."\n";
