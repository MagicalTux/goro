package tokenizer

import "strings"

func lexPhpPossibleCast(l *Lexer) lexState {
	// possible (string) etc
	l.pushState()

	l.advance(1) // "("
	l.acceptSpaces()

	typ := l.acceptPhpLabel()

	l.acceptSpaces()
	if !l.accept(")") {
		l.popState()
		return lexPhpOperator
	}

	switch strings.ToLower(typ) {
	case "int", "integer":
		l.emit(T_INT_CAST)
		return lexPhp
	case "bool", "boolean":
		l.emit(T_BOOL_CAST)
		return lexPhp
	case "float", "double", "real":
		l.emit(T_DOUBLE_CAST)
		return lexPhp
	case "string":
		l.emit(T_STRING_CAST)
		return lexPhp
	case "array":
		l.emit(T_ARRAY_CAST)
		return lexPhp
	case "object":
		l.emit(T_OBJECT_CAST)
		return lexPhp
	case "unset":
		l.emit(T_UNSET_CAST)
		return lexPhp
	}

	l.popState()
	return lexPhpOperator
}
