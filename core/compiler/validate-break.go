package compiler

import (
	"fmt"

	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// validateBreakContinue walks the AST to find break/continue statements that are
// not inside a loop or switch. loopDepth tracks the current nesting level.
// inFinally tracks whether we are inside a finally block.
// Returns a compile error if an invalid break/continue is found.
func validateBreakContinue(r phpv.Runnable, loopDepth int) error {
	return validateBreakContinueImpl(r, loopDepth, false)
}

func validateBreakContinueImpl(r phpv.Runnable, loopDepth int, inFinally bool) error {
	if r == nil {
		return nil
	}

	switch n := r.(type) {
	case *phperr.PhpBreak:
		if inFinally && int64(n.Initial) > int64(loopDepth) {
			return &phpv.PhpError{
				Err:  fmt.Errorf("jump out of a finally block is disallowed"),
				Loc:  n.L,
				Code: phpv.E_COMPILE_ERROR,
			}
		}
		if int64(n.Initial) > int64(loopDepth) {
			if loopDepth == 0 {
				return &phpv.PhpError{
					Err:  fmt.Errorf("'break' not in the 'loop' or 'switch' context"),
					Loc:  n.L,
					Code: phpv.E_COMPILE_ERROR,
				}
			}
			return &phpv.PhpError{
				Err:  fmt.Errorf("Cannot 'break' %d levels", n.Initial),
				Loc:  n.L,
				Code: phpv.E_COMPILE_ERROR,
			}
		}
		return nil
	case *phperr.PhpContinue:
		if inFinally && int64(n.Initial) > int64(loopDepth) {
			return &phpv.PhpError{
				Err:  fmt.Errorf("jump out of a finally block is disallowed"),
				Loc:  n.L,
				Code: phpv.E_COMPILE_ERROR,
			}
		}
		if int64(n.Initial) > int64(loopDepth) {
			if loopDepth == 0 {
				return &phpv.PhpError{
					Err:  fmt.Errorf("'continue' not in the 'loop' or 'switch' context"),
					Loc:  n.L,
					Code: phpv.E_COMPILE_ERROR,
				}
			}
			return &phpv.PhpError{
				Err:  fmt.Errorf("Cannot 'continue' %d levels", n.Initial),
				Loc:  n.L,
				Code: phpv.E_COMPILE_ERROR,
			}
		}
		return nil
	case *runGoto:
		if inFinally {
			return &phpv.PhpError{
				Err:  fmt.Errorf("jump out of a finally block is disallowed"),
				Loc:  n.l,
				Code: phpv.E_COMPILE_ERROR,
			}
		}
		return nil

	// Try/catch: finally blocks reset loop depth and set inFinally
	case *runnableTry:
		if err := validateBreakContinueImpl(n.try, loopDepth, inFinally); err != nil {
			return err
		}
		for _, c := range n.catches {
			if err := validateBreakContinueImpl(c.body, loopDepth, inFinally); err != nil {
				return err
			}
		}
		if n.finally != nil {
			// Inside a finally block, loop depth resets to 0 and inFinally is set
			if err := validateBreakContinueImpl(n.finally, 0, true); err != nil {
				return err
			}
		}
		return nil

	// Loop/switch constructs increase the depth
	case *runnableWhile:
		if err := validateBreakContinueImpl(n.cond, loopDepth, inFinally); err != nil {
			return err
		}
		return validateBreakContinueImpl(n.code, loopDepth+1, inFinally)
	case *runnableDoWhile:
		if err := validateBreakContinueImpl(n.code, loopDepth+1, inFinally); err != nil {
			return err
		}
		return validateBreakContinueImpl(n.cond, loopDepth, inFinally)
	case *runnableFor:
		if err := validateBreakContinueImpl(n.start, loopDepth, inFinally); err != nil {
			return err
		}
		if err := validateBreakContinueImpl(n.cond, loopDepth, inFinally); err != nil {
			return err
		}
		if err := validateBreakContinueImpl(n.each, loopDepth, inFinally); err != nil {
			return err
		}
		return validateBreakContinueImpl(n.code, loopDepth+1, inFinally)
	case *runnableForeach:
		if err := validateBreakContinueImpl(n.src, loopDepth, inFinally); err != nil {
			return err
		}
		return validateBreakContinueImpl(n.code, loopDepth+1, inFinally)
	case *runSwitch:
		if err := validateBreakContinueImpl(n.cond, loopDepth, inFinally); err != nil {
			return err
		}
		for _, b := range n.blocks {
			if err := validateBreakContinueImpl(b.cond, loopDepth+1, inFinally); err != nil {
				return err
			}
			if err := validateBreakContinueImpl(b.code, loopDepth+1, inFinally); err != nil {
				return err
			}
		}
		if n.def != nil {
			return validateBreakContinueImpl(n.def.code, loopDepth+1, inFinally)
		}
		return nil
	case *runMatch:
		if err := validateBreakContinueImpl(n.cond, loopDepth, inFinally); err != nil {
			return err
		}
		for _, arm := range n.arms {
			for _, cond := range arm.conditions {
				if err := validateBreakContinueImpl(cond, loopDepth, inFinally); err != nil {
					return err
				}
			}
			if err := validateBreakContinueImpl(arm.body, loopDepth, inFinally); err != nil {
				return err
			}
		}
		if n.def != nil {
			return validateBreakContinueImpl(n.def.body, loopDepth, inFinally)
		}
		return nil

	// Functions/closures create a new scope - don't recurse into them
	case *ZClosure:
		return nil
	case *phpobj.ZClass:
		return nil

	// For all other nodes, recurse into children
	default:
		for _, child := range GetChildren(r) {
			if err := validateBreakContinueImpl(child, loopDepth, inFinally); err != nil {
				return err
			}
		}
		return nil
	}
}
