package json

import (
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// > class JsonSerializable
var JsonSerializable = &phpobj.ZClass{
	Type: phpv.ZClassTypeInterface,
	Name: "JsonSerializable",
}

// > class JsonException extends Exception
var JsonException = &phpobj.ZClass{
	Name:    "JsonException",
	Extends: phpobj.Exception,
	Props:   phpobj.Exception.Props,
	Methods: phpobj.CopyMethods(phpobj.Exception.Methods),
}
