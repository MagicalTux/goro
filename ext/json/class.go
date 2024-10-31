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
