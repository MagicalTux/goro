package tokenizer

import "strings"

var phpMagicKeywords = map[string]ItemType{
	"abstract":        T_ABSTRACT,
	"array":           T_ARRAY,
	"as":              T_AS,
	"break":           T_BREAK,
	"callable":        T_CALLABLE,
	"case":            T_CASE,
	"catch":           T_CATCH,
	"class":           T_CLASS,
	"__CLASS__":       T_CLASS_C,
	"clone":           T_CLONE,
	"const":           T_CONST,
	"continue":        T_CONTINUE,
	"declare":         T_DECLARE,
	"default":         T_DEFAULT,
	"__DIR__":         T_DIR,
	"do":              T_DO,
	"echo":            T_ECHO,
	"else":            T_ELSE,
	"elseif":          T_ELSEIF,
	"empty":           T_EMPTY,
	"enddeclare":      T_ENDDECLARE,
	"endfor":          T_ENDFOR,
	"endforeach":      T_ENDFOREACH,
	"endif":           T_ENDIF,
	"endswitch":       T_ENDSWITCH,
	"endwhile":        T_ENDWHILE,
	"eval":            T_EVAL,
	"exit":            T_EXIT,
	"die":             T_EXIT,
	"extends":         T_EXTENDS,
	"__FILE__":        T_FILE,
	"final":           T_FINAL,
	"finally":         T_FINALLY,
	"for":             T_FOR,
	"foreach":         T_FOREACH,
	"function":        T_FUNCTION,
	"cfunction":       T_FUNCTION, // ?
	"__FUNCTION__":    T_FUNC_C,
	"global":          T_GLOBAL,
	"goto":            T_GOTO,
	"__halt_compiler": T_HALT_COMPILER,
	"if":              T_IF,
	"implements":      T_IMPLEMENTS,
	"include":         T_INCLUDE,
	"include_once":    T_INCLUDE_ONCE,
	"instanceof":      T_INSTANCEOF,
	"insteadof":       T_INSTEADOF,
	"interface":       T_INTERFACE,
	"isset":           T_ISSET,
	"__LINE__":        T_LINE,
	"list":            T_LIST,
	"and":             T_LOGICAL_AND,
	"or":              T_LOGICAL_OR,
	"xor":             T_LOGICAL_XOR,
	"__METHOD__":      T_METHOD_C,
	"namespace":       T_NAMESPACE,
	"__NAMESPACE__":   T_NS_C,
	"new":             T_NEW,
	"print":           T_PRINT,
	"private":         T_PRIVATE,
	"public":          T_PUBLIC,
	"protected":       T_PROTECTED,
	"require":         T_REQUIRE,
	"require_once":    T_REQUIRE_ONCE,
	"return":          T_RETURN,
	"static":          T_STATIC,
	"switch":          T_SWITCH,
	"throw":           T_THROW,
	"trait":           T_TRAIT,
	"__TRAIT__":       T_TRAIT_C,
	"try":             T_TRY,
	"unset":           T_UNSET,
	"use":             T_USE,
	"var":             T_VAR,
	"while":           T_WHILE,
	"yield":           T_YIELD,
	// yield from T_YIELD_FROM TODO special case
}

func lexPhpVariable(l *Lexer) lexState {
	l.advance(1) // '$' (already confirmed)
	if l.acceptPhpLabel() == "" {
		l.emit(ItemSingleChar)
		return l.base
	}

	l.emit(T_VARIABLE)
	return l.base
}

func labelType(lbl string) ItemType {
	// check for phpMagicKeywords
	if v, ok := phpMagicKeywords[strings.ToLower(lbl)]; ok {
		return v
	}
	if v, ok := phpMagicKeywords[lbl]; ok {
		return v
	}
	return T_STRING
}

func lexPhpString(l *Lexer) lexState {
	lbl := l.acceptPhpLabel()
	t := labelType(lbl)

	l.emit(t)
	if t == T_HALT_COMPILER {
		l.emit(itemEOF)
		return nil
	}
	return l.base
}
