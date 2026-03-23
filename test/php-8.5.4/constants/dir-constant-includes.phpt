--TEST--
__DIR__ constant test with includes
--FILE--
<?php
echo __DIR__ . "\n";
echo dirname(__FILE__) . "\n";
include 'fixtures/folder1/fixture.inc';
include 'fixtures/folder2/fixture.inc';
include 'fixtures/folder3/fixture.inc';
include 'fixtures/folder4/fixture.inc';
?>
--EXPECTF--
%stest%sconstants
%stest%sconstants
%stest%sconstants%sfixtures%sfolder1
%stest%sconstants%sfixtures%sfolder1
%stest%sconstants%sfixtures%sfolder2
%stest%sconstants%sfixtures%sfolder2
%stest%sconstants%sfixtures%sfolder3
%stest%sconstants%sfixtures%sfolder3
%stest%sconstants%sfixtures%sfolder4
%stest%sconstants%sfixtures%sfolder4
