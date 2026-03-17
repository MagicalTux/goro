package compiler

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

// a helper struct to make a Runnable parentable
type runnableChild struct {
	Parent phpv.Runnable
}

func (n *runnableChild) GetParentNode() phpv.Runnable {
	return n.Parent
}
func (n *runnableChild) SetParentNode(p phpv.Runnable) {
	n.Parent = p
}

type compileCtx interface {
	phpv.Context

	ExpectSingle(r rune) error
	NextItem() (*tokenizer.Item, error)
	backup()
	getClass() *phpobj.ZClass
	getFunc() *ZClosure
	peekType() tokenizer.ItemType
	getNamespace() phpv.ZString
	resolveClassName(name phpv.ZString) phpv.ZString
	resolveFunctionName(name phpv.ZString) phpv.ZString
	resolveConstantName(name string) string
}

type bracketEntry struct {
	char rune
	line int
}

type compileRootCtx struct {
	phpv.Context

	t *tokenizer.Lexer

	next             *tokenizer.Item
	last             *tokenizer.Item
	bracketStack     []bracketEntry
	lastBracketOp    int          // 0=none, 1=push, -1=pop
	lastBracketEntry bracketEntry // the entry that was pushed or popped

	namespace    phpv.ZString                  // current namespace (empty = global)
	useMap       map[phpv.ZString]phpv.ZString // use aliases for classes
	useFuncMap   map[phpv.ZString]phpv.ZString // use function aliases
	useConstMap  map[phpv.ZString]phpv.ZString // use const aliases
	nsClassNames map[phpv.ZString]bool         // short class names defined in current namespace
	strictTypes  bool                          // declare(strict_types=1) in effect
}

func (c *compileRootCtx) ExpectSingle(r rune) error {
	// read one item, check if rune, if not fallback & return error
	i, err := c.NextItem()
	if err != nil {
		return err
	}

	if !i.IsSingle(r) {
		c.backup()
		return i.Unexpected()
	}
	return nil
}

func (c *compileRootCtx) getNamespace() phpv.ZString {
	return c.namespace
}

// isBuiltinType checks if a name is a PHP built-in type that should not be namespace-resolved.
func isBuiltinType(lower phpv.ZString) bool {
	switch lower {
	case "self", "parent", "static",
		"int", "integer", "float", "double", "string", "bool", "boolean",
		"array", "callable", "void", "mixed", "never", "null", "object",
		"iterable", "true", "false":
		return true
	}
	return false
}

func (c *compileRootCtx) resolveClassName(name phpv.ZString) phpv.ZString {
	if len(name) == 0 {
		return name
	}
	// Fully qualified names start with \
	if name[0] == '\\' {
		return name[1:]
	}
	// Built-in types should never be resolved
	lower := name.ToLower()
	if isBuiltinType(lower) {
		return name
	}
	// Check if name contains a backslash (qualified name)
	for i := 0; i < len(name); i++ {
		if name[i] == '\\' {
			// Qualified name: resolve first part through use map, then append rest
			firstPart := name[:i]
			rest := name[i:] // includes the leading backslash
			if c.useMap != nil {
				if alias, ok := c.useMap[firstPart]; ok {
					return alias + rest
				}
			}
			// Not aliased — prepend current namespace
			if c.namespace != "" {
				return c.namespace + "\\" + name
			}
			return name
		}
	}
	// Unqualified name: check use map first
	if c.useMap != nil {
		if alias, ok := c.useMap[name]; ok {
			return alias
		}
	}
	// Prepend current namespace
	if c.namespace != "" {
		return c.namespace + "\\" + name
	}
	return name
}

func (c *compileRootCtx) resolveFunctionName(name phpv.ZString) phpv.ZString {
	if len(name) == 0 {
		return name
	}
	// Fully qualified names start with \
	if name[0] == '\\' {
		return name[1:]
	}
	// Check if name contains a backslash (qualified name)
	for i := 0; i < len(name); i++ {
		if name[i] == '\\' {
			// Qualified name: resolve first part through use map
			firstPart := name[:i]
			rest := name[i:]
			if c.useMap != nil {
				if alias, ok := c.useMap[firstPart]; ok {
					return alias + rest
				}
			}
			if c.namespace != "" {
				return c.namespace + "\\" + name
			}
			return name
		}
	}
	// Unqualified name: check use function map first
	if c.useFuncMap != nil {
		if alias, ok := c.useFuncMap[name]; ok {
			return alias
		}
	}
	// For functions, prepend namespace but also fall back to global at runtime.
	// We prepend the namespace here; GetFunction will try the global fallback.
	if c.namespace != "" {
		return c.namespace + "\\" + name
	}
	return name
}

func (c *compileRootCtx) resolveConstantName(name string) string {
	if len(name) == 0 {
		return name
	}
	// Fully qualified names start with \
	if name[0] == '\\' {
		return name[1:]
	}
	// Built-in constants should never be resolved
	if isBuiltinType(phpv.ZString(strings.ToLower(name))) {
		return name
	}
	// Check if name contains a backslash (qualified name)
	for i := 0; i < len(name); i++ {
		if name[i] == '\\' {
			// Qualified name: resolve first part through use map
			firstPart := phpv.ZString(name[:i])
			rest := name[i:]
			if c.useMap != nil {
				if alias, ok := c.useMap[firstPart]; ok {
					return string(alias) + rest
				}
			}
			if c.namespace != "" {
				return string(c.namespace) + "\\" + name
			}
			return name
		}
	}
	// Unqualified name: check use const map first
	if c.useConstMap != nil {
		if alias, ok := c.useConstMap[phpv.ZString(name)]; ok {
			return string(alias)
		}
	}
	// For constants, prepend namespace but also fall back to global at runtime.
	if c.namespace != "" {
		return string(c.namespace) + "\\" + name
	}
	return name
}

func (c *compileRootCtx) getClass() *phpobj.ZClass {
	return nil
}

func (c *compileRootCtx) getFunc() *ZClosure {
	return nil
}

func (c *compileRootCtx) peekType() tokenizer.ItemType {
	if c.next != nil {
		return c.next.Type
	}

	// Read directly from tokenizer, bypassing bracket tracking.
	// The token is stored in c.next and will be bracket-tracked
	// when NextItem actually consumes it.
	for {
		i, err := c.t.NextItem()
		if err != nil {
			return -1
		}
		switch i.Type {
		case tokenizer.T_WHITESPACE, tokenizer.T_COMMENT:
			continue
		default:
			c.next = i
			return i.Type
		}
	}
}

func (c *compileRootCtx) NextItem() (*tokenizer.Item, error) {
	var i *tokenizer.Item
	if c.next != nil {
		i = c.next
		c.next = nil
	} else {
		for {
			var err error
			i, err = c.t.NextItem()
			if err != nil {
				return i, err
			}
			if i.Type != tokenizer.T_WHITESPACE && i.Type != tokenizer.T_COMMENT {
				break
			}
		}
	}

	c.last = i

	// Track brackets for better syntax error messages
	if bracketErr := c.trackBracket(i); bracketErr != nil {
		return i, bracketErr
	}

	return i, nil
}

var matchingBracket = map[rune]rune{')': '(', ']': '[', '}': '{'}

func (c *compileRootCtx) trackBracket(i *tokenizer.Item) error {
	c.lastBracketOp = 0 // reset

	switch {
	case i.IsSingle('('):
		entry := bracketEntry{'(', i.Line}
		c.bracketStack = append(c.bracketStack, entry)
		c.lastBracketOp = 1
		c.lastBracketEntry = entry
	case i.IsSingle('['), i.Type == tokenizer.T_ATTRIBUTE:
		// T_ATTRIBUTE represents #[ which opens a bracket like [
		entry := bracketEntry{'[', i.Line}
		c.bracketStack = append(c.bracketStack, entry)
		c.lastBracketOp = 1
		c.lastBracketEntry = entry
	case i.IsSingle('{'):
		entry := bracketEntry{'{', i.Line}
		c.bracketStack = append(c.bracketStack, entry)
		c.lastBracketOp = 1
		c.lastBracketEntry = entry
	case i.IsSingle(')'), i.IsSingle(']'), i.IsSingle('}'):
		var closingChar rune
		if i.IsSingle(')') {
			closingChar = ')'
		} else if i.IsSingle(']') {
			closingChar = ']'
		} else {
			closingChar = '}'
		}
		expected := matchingBracket[closingChar]
		if len(c.bracketStack) == 0 {
			return c.bracketError(fmt.Sprintf("Unmatched '%c'", closingChar), i)
		}
		top := c.bracketStack[len(c.bracketStack)-1]
		if top.char != expected {
			msg := fmt.Sprintf("Unclosed '%c'", top.char)
			if top.line != i.Line {
				msg += fmt.Sprintf(" on line %d", top.line)
			}
			msg += fmt.Sprintf(" does not match '%c'", closingChar)
			return c.bracketError(msg, i)
		}
		c.lastBracketEntry = top // save what we're popping for undo
		c.bracketStack = c.bracketStack[:len(c.bracketStack)-1]
		c.lastBracketOp = -1
	case i.Type == tokenizer.T_EOF:
		if len(c.bracketStack) > 0 {
			top := c.bracketStack[len(c.bracketStack)-1]
			msg := fmt.Sprintf("Unclosed '%c'", top.char)
			if top.line != i.Line {
				msg += fmt.Sprintf(" on line %d", top.line)
			}
			return c.bracketError(msg, i)
		}
	}
	return nil
}

func (c *compileRootCtx) bracketError(msg string, i *tokenizer.Item) error {
	return &phpv.PhpError{
		Err:          fmt.Errorf("%s", msg),
		Code:         phpv.E_PARSE,
		Loc:          i.Loc(),
		GoStackTrace: phpv.GetGoDebugTrace(),
	}
}

func (c *compileRootCtx) backup() {
	// Undo the last bracket tracking operation
	switch c.lastBracketOp {
	case 1: // was a push, undo by popping
		if len(c.bracketStack) > 0 {
			c.bracketStack = c.bracketStack[:len(c.bracketStack)-1]
		}
	case -1: // was a pop, undo by pushing back
		c.bracketStack = append(c.bracketStack, c.lastBracketEntry)
	}
	c.lastBracketOp = 0
	c.next, c.last = c.last, nil
}

func init() {
	phpctx.Compile = Compile
}

func Compile(parent phpv.Context, t *tokenizer.Lexer) (phpv.Runnable, error) {
	// Check if the global context has a deadline. If so, enforce it on compilation
	// to prevent tokenizer/compiler deadlocks from hanging forever.
	if deadline, ok := parent.Global().Deadline(); ok {
		timeout := time.Until(deadline)
		if timeout <= 0 {
			t.Close()
			return nil, fmt.Errorf("compile deadline already expired")
		}
		type result struct {
			r   phpv.Runnable
			err error
		}
		ch := make(chan result, 1)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					ch <- result{nil, fmt.Errorf("compile panic: %v", r)}
				}
			}()
			r, err := compileInner(parent, t)
			ch <- result{r, err}
		}()
		select {
		case res := <-ch:
			return res.r, res.err
		case <-time.After(timeout):
			t.Close()
			return nil, fmt.Errorf("compile timed out")
		}
	}

	return compileInner(parent, t)
}

func compileInner(parent phpv.Context, t *tokenizer.Lexer) (phpv.Runnable, error) {
	c := &compileRootCtx{
		Context:      parent,
		t:            t,
		useMap:       make(map[phpv.ZString]phpv.ZString),
		useFuncMap:   make(map[phpv.ZString]phpv.ZString),
		useConstMap:  make(map[phpv.ZString]phpv.ZString),
		nsClassNames: make(map[phpv.ZString]bool),
	}

	r, err := compileBaseUntil(nil, c, tokenizer.T_EOF)
	if err != nil && err != io.EOF {
		return nil, err
	}

	if list, ok := r.(phpv.Runnables); ok {
		// check for any function
		for i, elem := range list {
			switch obj := elem.(type) {
			case *ZClosure:
				if obj.name != "" {
					c.Global().RegisterLazyFunc(obj.name, list, i)
				}
			case *phpobj.ZClass:
				// TODO first index classes by name, instanciate in right order
				if obj.Name != "" {
					c.Global().RegisterLazyClass(obj.Name, list, i)
				}
			}
		}
	}

	// Validate break/continue at the top-level scope (outside loops).
	// Functions are validated in compileFunctionWithName; this catches
	// break/continue in the global scope.
	if breakErr := validateBreakContinue(r, 0); breakErr != nil {
		return nil, breakErr
	}

	// In some cases, a node needs to know if it's a write context,
	// and one way of conveying that information is with parent nodes.
	// Doing something like ctx = WriteContext(ctx) wouldn't work
	// since the surrounding context isn't known yet and expressions
	// are parsed from left to right.
	ConnectParentNodes(r)

	return r, nil
}

func ConnectParentNodes(r phpv.Runnable) {
	if r == nil {
		return
	}
	for _, c := range GetChildren(r) {
		if c == nil {
			continue
		}
		if rc, ok := c.(phpv.RunnableChild); ok {
			rc.SetParentNode(r)
		}
		ConnectParentNodes(c)
	}
}

// TODO: probably better to go-generate instead
func GetChildren(r phpv.Runnable) []phpv.Runnable {
	type rt []phpv.Runnable
	switch t := r.(type) {
	case *runRef:
		return rt{t.v}
	case *runVariable:
		return nil
	case *runVariableRef:
		return rt{t.v}
	case *runParentheses:
		return rt{t.r}
	case *runZVal:
		if x, ok := t.v.(phpv.Runnable); ok {
			return rt{x}
		}
		return nil
	case *runOperator:
		return rt{t.a, t.b}
	case *runConstant:
		return nil
	case runConcat:
		return t
	case *runnableWhile:
		return rt{t.cond, t.code}
	case *runnableDoWhile:
		return rt{t.cond, t.code}
	case *runnableIf:
		return rt{t.cond, t.yes, t.no}
	case *runnableFor:
		return rt{t.start, t.cond, t.each, t.code}
	case *runnableForeach:
		return rt{t.src, t.code, t.k, t.v}
	case *runSwitch:
		res := rt{t.cond}
		if t.def != nil {
			res = append(res, t.def.cond)
		}
		for _, e := range t.blocks {
			res = append(res, e.cond)
			res = append(res, e.code)
		}
		return res
	case *runReturn:
		return rt{t.v}
	case *runnableTry:
		res := rt{t.try, t.finally}
		for _, e := range t.catches {
			res = append(res, e.body)
		}
		return res
	case *runnableThrow:
		return rt{t.v}
	case runInlineHtml:
		return nil
	case *runStaticVar:
		res := rt{}
		for _, e := range t.vars {
			res = append(res, e.def)
		}
		return res
	case *runObjectFunc:
		res := rt{t.ref}
		for _, e := range t.args {
			res = append(res, e)
		}
		return res
	case *runObjectVar:
		return rt{t.ref}
	case *runNewObject:
		res := rt{t.cl}
		for _, e := range t.newArg {
			res = append(res, e)
		}
		return res
	case *runnableIsset:
		res := rt{}
		for _, e := range t.args {
			res = append(res, e)
		}
		return res
	case *runInstanceOf:
		return rt{t.v}
	case *runGlobal:
		return nil
	case *runnableFunctionCall:
		res := rt{}
		for _, e := range t.args {
			res = append(res, e)
		}
		return res
	case *runnableFunctionCallRef:
		res := rt{t.name}
		for _, e := range t.args {
			res = append(res, e)
		}
		return res
	case *runDestructure:
		res := rt{}
		for _, e := range t.e {
			res = append(res, e.k)
			res = append(res, e.v)
		}
		return res
	case *runnableClone:
		return rt{t.arg}
	case *NamedArg:
		return rt{t.Arg}
	case *runClassStaticVarRef:
		return rt{t.className}
	case *runClassStaticObjRef:
		return rt{t.className}
	case *runArray:
		res := rt{}
		for _, e := range t.e {
			res = append(res, e.k)
			res = append(res, e.v)
		}
		return res
	case *runArrayAccess:
		return []phpv.Runnable{t.offset, t.value}
	case phpv.Runnables:
		return t
	case *ZClosure:
		return rt{t.code}
	case *phpobj.ZClass:
		res := rt{}
		for _, v := range t.Methods {
			if f, ok := v.Method.(phpv.Runnable); ok {
				res = append(res, f)
			}
		}
		return res
	case *runnableUnset:
		return t.args
	case *runNoDiscardStatement:
		return rt{t.inner}
	case *runDestroyTemporary:
		return rt{t.inner}
	case *phperr.PhpBreak:
		return nil
	case *phperr.PhpContinue:
		return nil
	case *runYield:
		res := rt{}
		if t.key != nil {
			res = append(res, t.key)
		}
		if t.value != nil {
			res = append(res, t.value)
		}
		return res
	case *runYieldFrom:
		return rt{t.expr}
	default:
		// Unknown node type — return empty children rather than panicking
		return nil
	}
}
