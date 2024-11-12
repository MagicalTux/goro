package core

import (
	"runtime"

	"github.com/MagicalTux/goro/core/phpv"
)

// TODO improve these

// > const
const (
	PHP_OS        = phpv.ZString(runtime.GOOS)
	PHP_OS_FAMILY = phpv.ZString(runtime.GOOS)
)
