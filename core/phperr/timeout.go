package phperr

import (
	"fmt"

	"github.com/MagicalTux/goro/core/phpv"
)

type PhpTimeout struct {
	L       *phpv.Loc
	Seconds int
}

func (t *PhpTimeout) Error() string {
	suffix := ""
	if t.Seconds > 1 {
		suffix = "s"
	}
	return fmt.Sprintf("Maximum execution time of %d second%s exceeded", t.Seconds, suffix)
}

func (t *PhpTimeout) String() string {
	return fmt.Sprintf("Fatal error: %s in %s on line %d\n\n", t.Error(), t.L.Filename, t.L.Line)
}

func CatchTimeout(err error) error {
	if err == nil {
		return nil
	}
	switch err := err.(type) {
	case *PhpTimeout:
		return err
	case *phpv.PhpError:
		switch err := err.Err.(type) {
		case *PhpTimeout:
			return err
		}
	}
	return nil
}
