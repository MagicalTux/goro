package core

type runInlineHtml string

func (s runInlineHtml) run(ctx Context) (*ZVal, error) {
	_, err := ctx.Write([]byte(s))
	return nil, err
}
