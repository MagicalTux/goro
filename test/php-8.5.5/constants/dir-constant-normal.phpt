--TEST--
Standard behaviour of __DIR__
--FILE--
<?php
echo __DIR__ . "\n";
echo dirname(__FILE__) . "\n";
?>
--EXPECTF--
%stest%sconstants
%stest%sconstants
