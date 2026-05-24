package vmcompiler

import (
	"github.com/KarpelesLab/goro/core/compiler"
	"github.com/KarpelesLab/goro/core/vm"
)

// emitTry lowers a `try { … } catch (T1|T2 $e) { … } [finally { … }]`
// to bytecode.
//
// Layout WITHOUT finally:
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
// Layout WITH finally:
//
//	tryStart:
//	   <try body>
//	   JMP finallyPC           // normal completion: pending=none, fall through finally
//	tryEnd:
//	catch_1:
//	   <body>
//	   JMP finallyPC
//	catch_2:
//	   ...
//	finallyPC:                 // == afterCatch
//	   <finally body>
//	   OP_FINALLY_END          // inspects f.pending: re-raise / re-return / fall through
//	finallyEnd:                // (post-finally code)
//
// A TryHandler with Start=tryStart, End=tryEnd, and (when present)
// HasFinally + FinallyPC + FinallyEnd is registered on the function.
// The dispatcher routes PhpThrow through the catches when failPC is in
// the try body; if no catch matches (or the throw originates inside a
// catch body) and HasFinally, the throw is parked in f.pending and
// execution jumps to FinallyPC. OpRet / OpRetNull do the same for
// returns whose PC is enclosed by a finally region.
func (e *emitter) emitTry(n compiler.TryNode) error {
	finally := n.TryFinally()
	catches := n.TryCatches()
	if len(catches) == 0 && finally == nil {
		// Bare try with no catch and no finally is a parse error in
		// PHP, but be defensive.
		return unsupportedf("try without catch or finally")
	}

	tryStart := uint32(len(e.code))
	stackBase := e.curStack

	// Track this finally's emit-time loop depth so cross-finally
	// break/continue can be rejected at emit time (the JMPs they'd
	// emit don't route through pending+finally yet).
	if finally != nil {
		e.finallyLoopDepths = append(e.finallyLoopDepths, len(e.loops))
	}
	if err := e.emitStmt(n.TryBody()); err != nil {
		return err
	}
	// Normal-completion JMP: lands at finallyPC if there's a finally,
	// otherwise at afterCatch. Either way it's the same PC because we
	// arrange the finally body to start exactly at afterCatch.
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
		endPatches = append(endPatches, e.emit(vm.OpJmp, 0, 0, 0))
	}

	// All catches done: this PC is afterCatch — and, when there's a
	// finally, also finallyPC (the finally body begins here so a
	// natural fall-through from try / catch lands directly in it).
	afterCatch := uint32(len(e.code))
	e.patchJump(skipCatches, afterCatch)
	for _, pc := range endPatches {
		e.patchJump(pc, afterCatch)
	}

	handler := vm.TryHandler{
		Start:      tryStart,
		End:        tryEnd,
		AfterCatch: afterCatch,
		StackBase:  stackBase,
		Catches:    clauses,
	}

	if finally != nil {
		// Pop the finally tracking now: the finally body itself runs
		// at the normal loop depth, so a break inside finally that
		// targets a loop entered inside the finally body is fine.
		e.finallyLoopDepths = e.finallyLoopDepths[:len(e.finallyLoopDepths)-1]
		handler.HasFinally = true
		handler.FinallyPC = afterCatch
		if err := e.emitStmt(finally); err != nil {
			return err
		}
		// OP_FINALLY_END's A field is this handler's index in
		// Function.TryHandlers (assigned just below) — the runtime
		// reads f.handlerPending[A] to recover this finally's parked
		// action and clears the slot. We append the handler after
		// emitting, so the index is the current length.
		hidx := len(e.tryHandlers)
		if hidx > 0xFFFF {
			return unsupportedf("more than 65535 try handlers in one function")
		}
		finallyEndPC := e.emit(vm.OpFinallyEnd, uint16(hidx), 0, 0)
		handler.FinallyEnd = finallyEndPC
	}

	e.tryHandlers = append(e.tryHandlers, handler)
	return nil
}
