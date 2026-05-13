package vmcompiler

import (
	"github.com/KarpelesLab/goro/core/compiler"
	"github.com/KarpelesLab/goro/core/vm"
)

// emitTry lowers a `try { … } catch (T1|T2 $e) { … } …` to bytecode.
// finally clauses currently fall back via ErrUnsupported.
//
// Layout:
//
//	tryStart:
//	   <try body>
//	   JMP afterCatch          // skip catches on normal completion
//	tryEnd:                    // also = first catch PC
//	catch_1:
//	   <body>
//	   JMP afterCatch
//	catch_2:
//	   ...
//	afterCatch:
//
// A TryHandler with Start=tryStart, End=tryEnd, and one CatchClause
// per catch is registered on the function. The dispatcher catches
// PhpThrow within [tryStart, tryEnd) and redirects pc to the first
// matching catch.PC.
func (e *emitter) emitTry(n compiler.TryNode) error {
	if n.TryFinally() != nil {
		return unsupportedf("try with finally")
	}
	catches := n.TryCatches()
	if len(catches) == 0 {
		// Bare try with no catch and no finally is a parse error in
		// PHP, but be defensive.
		return unsupportedf("try without catch or finally")
	}

	tryStart := uint32(len(e.code))
	stackBase := e.curStack
	if err := e.emitStmt(n.TryBody()); err != nil {
		return err
	}
	// Skip catches on normal completion.
	skipCatches := e.emit(vm.OpJmp, 0, 0, 0)
	tryEnd := uint32(len(e.code))

	clauses := make([]vm.CatchClause, len(catches))
	endPatches := make([]uint32, 0, len(catches))
	for i, c := range catches {
		clauses[i].PC = uint32(len(e.code))
		clauses[i].Types = c.CatchTypes()
		clauses[i].Loc = c.CatchLoc()
		if name := c.CatchVarName(); name != "" {
			clauses[i].VarIdx = e.localIndex(name)
		} else {
			clauses[i].VarIdx = 0xFFFF
		}
		if err := e.emitStmt(c.CatchBody()); err != nil {
			return err
		}
		// Skip remaining catches.
		endPatches = append(endPatches, e.emit(vm.OpJmp, 0, 0, 0))
	}

	afterCatch := uint32(len(e.code))
	e.patchJump(skipCatches, afterCatch)
	for _, pc := range endPatches {
		e.patchJump(pc, afterCatch)
	}

	e.tryHandlers = append(e.tryHandlers, vm.TryHandler{
		Start:      tryStart,
		End:        tryEnd,
		AfterCatch: afterCatch,
		StackBase:  stackBase,
		Catches:    clauses,
	})
	return nil
}
