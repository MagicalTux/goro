# Purpose

The idea is to be compatible with most scripts written for PHP while taking advantage of Go's features and provide various performance improvements or take shortcuts where no compatibility is needed.

# Differences

* In this version, echo is a function. Most special function calling keywords (echo, exit, die, etc) are actually just calling functions under the same name.
* Object destructors are never called
