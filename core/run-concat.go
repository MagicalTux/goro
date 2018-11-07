package core

type runConcat []runnable

func (r runConcat) run(ctx Context) (l *ZVal, err error) {
	res := ""
	var t *ZVal

	for _, v := range r {
		t, err = v.run(ctx)
		if err != nil {
			return
		}
		res = res + t.String()
	}
	l = &ZVal{ZString(res)}
	return
}
