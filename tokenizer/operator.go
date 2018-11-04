package tokenizer

var lexPhpOps = map[string]ItemType{
	"&=":  T_AND_EQUAL,
	"&&":  T_BOOLEAN_AND,
	"||":  T_BOOLEAN_OR,
	"??":  T_COALESCE,
	".=":  T_CONCAT_EQUAL,
	"--":  T_DEC,
	"++":  T_INC,
	"/=":  T_DIV_EQUAL,
	"=>":  T_DOUBLE_ARROW,
	"::":  T_PAAMAYIM_NEKUDOTAYIM,
	"...": T_ELLIPSIS,
	"==":  T_IS_EQUAL,
	">=":  T_IS_GREATER_OR_EQUAL,
	"===": T_IS_IDENTICAL,
	"!=":  T_IS_NOT_EQUAL,
	"<>":  T_IS_NOT_EQUAL,
	"!==": T_IS_NOT_IDENTICAL,
	"<=":  T_IS_SMALLER_OR_EQUAL,
	"<=>": T_SPACESHIP,
	"-=":  T_MINUS_EQUAL,
	"%=":  T_MOD_EQUAL,
	"*=":  T_MUL_EQUAL,
	"->":  T_OBJECT_OPERATOR,
	"|=":  T_OR_EQUAL,
	"+=":  T_PLUS_EQUAL,
	"**":  T_POW,
	"**=": T_POW_EQUAL,
	"<<":  T_SL,
	"<<=": T_SL_EQUAL,
	">>":  T_SR,
	">>=": T_SR_EQUAL,
	"<<<": T_START_HEREDOC, // TODO
	"^=":  T_XOR_EQUAL,
}

func lexPhpOperator(l *Lexer) lexState {
	if t, ok := lexPhpOps[l.peekString(3)]; ok {
		l.advance(3)
		l.emit(t)
		return lexPhp
	}

	if t, ok := lexPhpOps[l.peekString(2)]; ok {
		l.advance(2)
		l.emit(t)
		return lexPhp
	}

	l.advance(1)
	l.emit(ItemSingleChar)
	return lexPhp
}
