package date

import (
	"time"

	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

var DateTime *phpobj.ZClass

func init() {
	DateTime = &phpobj.ZClass{
		Name:  "DateTime",
		Props: []*phpv.ZClassProp{},
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"__construct": {
				Name:      "__construct",
				Modifiers: phpv.ZAttrPublic,
				Method: phpobj.NativeMethod(func(ctx phpv.Context, this *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
					// Store the time as opaque data
					var t time.Time
					if len(args) > 0 && !args[0].IsNull() {
						dateStr := args[0].AsString(ctx)
						// Try common formats
						for _, layout := range []string{
							"2006-01-02 15:04:05 MST",
							"2006-01-02 15:04:05",
							"2006-01-02",
							time.RFC3339,
						} {
							if parsed, err := time.Parse(layout, string(dateStr)); err == nil {
								t = parsed
								break
							}
						}
						if t.IsZero() {
							t = time.Now()
						}
					} else {
						t = time.Now()
					}
					this.Opaque[DateTime] = t
					return nil, nil
				}),
			},
		},
	}
}
