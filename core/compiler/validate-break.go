package compiler

import (
	"fmt"

	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// validateBreakContinue walks the AST to find break/continue statements that are
// not inside a loop or switch. loopDepth tracks the current nesting level.
// Returns a compile error if an invalid break/continue is found.
func validateBreakContinue(r phpv.Runnable, loopDepth int) error {
	if r == nil {
		return nil
	}

	switch n := r.(type) {
	case *phperr.PhpBreak:
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

	// Loop/switch constructs increase the depth
	case *runnableWhile:
		if err := validateBreakContinue(n.cond, loopDepth); err != nil {
			return err
		}
		return validateBreakContinue(n.code, loopDepth+1)
	case *runnableDoWhile:
		if err := validateBreakContinue(n.code, loopDepth+1); err != nil {
			return err
		}
		return validateBreakContinue(n.cond, loopDepth)
	case *runnableFor:
		if err := validateBreakContinue(n.start, loopDepth); err != nil {
			return err
		}
		if err := validateBreakContinue(n.cond, loopDepth); err != nil {
			return err
		}
		if err := validateBreakContinue(n.each, loopDepth); err != nil {
			return err
		}
		return validateBreakContinue(n.code, loopDepth+1)
	case *runnableForeach:
		if err := validateBreakContinue(n.src, loopDepth); err != nil {
			return err
		}
		return validateBreakContinue(n.code, loopDepth+1)
	case *runSwitch:
		if err := validateBreakContinue(n.cond, loopDepth); err != nil {
			return err
		}
		for _, b := range n.blocks {
			if err := validateBreakContinue(b.cond, loopDepth+1); err != nil {
				return err
			}
			if err := validateBreakContinue(b.code, loopDepth+1); err != nil {
				return err
			}
		}
		if n.def != nil {
			return validateBreakContinue(n.def.code, loopDepth+1)
		}
		return nil
	case *runMatch:
		if err := validateBreakContinue(n.cond, loopDepth); err != nil {
			return err
		}
		for _, arm := range n.arms {
			for _, cond := range arm.conditions {
				if err := validateBreakContinue(cond, loopDepth); err != nil {
					return err
				}
			}
			if err := validateBreakContinue(arm.body, loopDepth); err != nil {
				return err
			}
		}
		if n.def != nil {
			return validateBreakContinue(n.def.body, loopDepth)
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
			if err := validateBreakContinue(child, loopDepth); err != nil {
				return err
			}
		}
		return nil
	}
}
