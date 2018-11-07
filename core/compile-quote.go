package core

import "git.atonline.com/tristantech/gophp/core/tokenizer"

func compileQuoteEncapsed(i *tokenizer.Item, c *compileCtx) (runnable, error) {
	// i == '"'

	var res runConcat
	var err error

	for {
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}

		_ = res
		switch i.Type {
		case tokenizer.T_ENCAPSED_AND_WHITESPACE:
			// TODO unescape string
			res = append(res, &ZVal{ZString(i.Data)})
		case tokenizer.T_VARIABLE:
			res = append(res, runVariable(i.Data[1:]))
		case tokenizer.ItemSingleChar:
			switch []rune(i.Data)[0] {
			case '"':
				// end of quote
				return res, nil
			}
		default:
			return nil, i.Unexpected()
		}
	}
}
