package json

import "fmt"

func (e JsonError) Error() string {
	return fmt.Sprintf("Json Error %s", e.String())
}
