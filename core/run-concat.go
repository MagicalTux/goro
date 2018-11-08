package core

type runConcat []Runnable

func (r runConcat) Run(ctx Context) (l *ZVal, err error) {
	res := ""
	var t *ZVal

	for _, v := range r {
		t, err = v.Run(ctx)
		if err != nil {
			return
		}
		res = res + t.String()
	}
	l = &ZVal{ZString(res)}
	return
}
