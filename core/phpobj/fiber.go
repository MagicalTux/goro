package phpobj

import (
	"context"
	"fmt"
	"time"

	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpv"
)

// FiberStatus tracks the state of a Fiber.
type FiberStatus int

const (
	FiberCreated    FiberStatus = iota // constructed but not started
	FiberRunning                       // currently executing
	FiberSuspended                     // suspended via Fiber::suspend()
	FiberTerminated                    // finished (returned or threw)
)

// fiberContextKey is used to store the current FiberState in the context.
type fiberContextKey struct{}

// fiberObjectKey is used to store the current Fiber ZObject in the context.
type fiberObjectKey struct{}

// FiberResolveCallable is set by the core package to resolve a ZVal into a Callable.
// This avoids an import cycle between phpobj and core.
var FiberResolveCallable func(ctx phpv.Context, v *phpv.ZVal) (phpv.Callable, error)

// FiberState holds the internal state of a PHP Fiber.
type FiberState struct {
	callback phpv.Callable
	status   FiberStatus

	// Channels for cooperative scheduling between caller and fiber goroutine.
	// The protocol ensures mutual exclusion: at any point, exactly one side is running.
	resumeCh  chan fiberMsg // caller -> fiber: value sent via resume()/start()
	suspendCh chan fiberMsg // fiber -> caller: value sent via Fiber::suspend() or termination

	returnVal *phpv.ZVal // final return value
	fiberErr  error       // error from fiber (uncaught exception)
}

// fiberMsg carries a value or an error between the caller and fiber goroutine.
type fiberMsg struct {
	val *phpv.ZVal
	err error // non-nil means "throw this inside the fiber"
}

// fiberExecContext wraps a phpv.Context to carry the FiberState via Go's context.Value.
type fiberExecContext struct {
	phpv.Context
	goCtx context.Context
}

func (f *fiberExecContext) Deadline() (deadline time.Time, ok bool) {
	return f.goCtx.Deadline()
}

func (f *fiberExecContext) Done() <-chan struct{} {
	return f.goCtx.Done()
}

func (f *fiberExecContext) Err() error {
	return f.goCtx.Err()
}

func (f *fiberExecContext) Value(key any) any {
	if v := f.goCtx.Value(key); v != nil {
		return v
	}
	return f.Context.Value(key)
}

// FiberError class - extends Error
var FiberError *ZClass

// Fiber class - final class
var Fiber *ZClass

func initFiberClasses() {
	FiberError = &ZClass{
		Name:    "FiberError",
		Extends: Error,
		Props:   Error.Props,
		Methods: CopyMethods(Error.Methods),
	}

	Fiber = &ZClass{
		Name: "Fiber",
		Attr: phpv.ZClassAttr(phpv.ZClassFinal),
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"__construct":  {Name: "__construct", Method: NativeMethod(fiberConstruct)},
			"start":        {Name: "start", Method: NativeMethod(fiberStart)},
			"resume":       {Name: "resume", Method: NativeMethod(fiberResume)},
			"getreturn":    {Name: "getReturn", Method: NativeMethod(fiberGetReturn)},
			"isstarted":    {Name: "isStarted", Method: NativeMethod(fiberIsStarted)},
			"isrunning":    {Name: "isRunning", Method: NativeMethod(fiberIsRunning)},
			"issuspended":  {Name: "isSuspended", Method: NativeMethod(fiberIsSuspended)},
			"isterminated": {Name: "isTerminated", Method: NativeMethod(fiberIsTerminated)},
			"throw":        {Name: "throw", Modifiers: phpv.ZAttrPublic, Method: NativeMethod(fiberThrow)},
			"suspend": {Name: "suspend", Modifiers: phpv.ZAttrPublic | phpv.ZAttrStatic,
				Method: NativeStaticMethod(fiberSuspend)},
			"getcurrent": {Name: "getCurrent", Modifiers: phpv.ZAttrPublic | phpv.ZAttrStatic,
				Method: NativeStaticMethod(fiberGetCurrent)},
		},
	}
}

func getFiberState(o *ZObject) *FiberState {
	opaque := o.GetOpaque(Fiber)
	if opaque == nil {
		return nil
	}
	return opaque.(*FiberState)
}

// fiberConstruct implements Fiber::__construct(callable $callback)
func fiberConstruct(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, ThrowError(ctx, TypeError, "Fiber::__construct() expects exactly 1 argument, 0 given")
	}

	// The callback is stored as a ZVal; we'll resolve it to a Callable when start() is called.
	// This avoids issues with callable resolution at construction time.
	state := &FiberState{
		status: FiberCreated,
	}
	o.SetOpaque(Fiber, state)

	// Store the callback ZVal for later resolution
	o.HashTable().SetString("__fiber_callback", args[0])

	return nil, nil
}

// fiberStart implements $fiber->start(mixed ...$args): mixed
func fiberStart(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	state := getFiberState(o)
	if state == nil {
		return nil, ThrowError(ctx, FiberError, "Cannot start a fiber that is not properly initialized")
	}
	if state.status != FiberCreated {
		return nil, ThrowError(ctx, FiberError, "Cannot start a fiber that is not in the created state")
	}

	// Resolve the callback
	callbackZVal := o.HashTable().GetString("__fiber_callback")
	if callbackZVal == nil {
		return nil, ThrowError(ctx, FiberError, "Fiber callback not set")
	}

	// Try direct Callable extraction first, then use the resolver
	var callback phpv.Callable
	if c, ok := callbackZVal.Value().(phpv.Callable); ok {
		callback = c
	} else if FiberResolveCallable != nil {
		var err error
		callback, err = FiberResolveCallable(ctx, callbackZVal)
		if err != nil {
			return nil, ThrowError(ctx, TypeError, fmt.Sprintf("Fiber::__construct() expects parameter 1 to be a valid callback, %s", err))
		}
	} else {
		return nil, ThrowError(ctx, TypeError, "Fiber::__construct() expects parameter 1 to be a valid callback")
	}
	state.callback = callback

	state.resumeCh = make(chan fiberMsg, 0)
	state.suspendCh = make(chan fiberMsg, 0)
	state.status = FiberRunning

	// Launch the fiber goroutine
	go func() {
		// Create a context that carries the fiber state and the fiber object
		goCtx := context.WithValue(ctx, fiberContextKey{}, state)
		goCtx = context.WithValue(goCtx, fiberObjectKey{}, o)
		fiberCtx := &fiberExecContext{
			Context: ctx,
			goCtx:   goCtx,
		}

		// Call the callback
		result, err := fiberCtx.CallZVal(fiberCtx, callback, args)

		// Fiber completed
		state.status = FiberTerminated
		if err != nil {
			state.fiberErr = err
			state.suspendCh <- fiberMsg{err: err}
		} else {
			state.returnVal = result
			state.suspendCh <- fiberMsg{val: nil} // nil val signals termination
		}
	}()

	// Wait for the fiber to suspend or terminate
	msg := <-state.suspendCh

	if state.status == FiberTerminated {
		if msg.err != nil {
			// Re-throw the fiber's exception in the caller
			return nil, msg.err
		}
		return phpv.ZNULL.ZVal(), nil
	}

	// Fiber suspended — return the suspended value
	return msg.val, nil
}

// fiberResume implements $fiber->resume(mixed $value = null): mixed
func fiberResume(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	state := getFiberState(o)
	if state == nil {
		return nil, ThrowError(ctx, FiberError, "Cannot resume a fiber that is not properly initialized")
	}
	if state.status != FiberSuspended {
		return nil, ThrowError(ctx, FiberError, "Cannot resume a fiber that is not suspended")
	}

	var resumeVal *phpv.ZVal
	if len(args) > 0 {
		resumeVal = args[0]
	} else {
		resumeVal = phpv.ZNULL.ZVal()
	}

	state.status = FiberRunning

	// Send the resume value to the fiber
	state.resumeCh <- fiberMsg{val: resumeVal}

	// Wait for the fiber to suspend or terminate
	msg := <-state.suspendCh

	if state.status == FiberTerminated {
		if msg.err != nil {
			return nil, msg.err
		}
		return phpv.ZNULL.ZVal(), nil
	}

	return msg.val, nil
}

// fiberSuspend implements Fiber::suspend(mixed $value = null): mixed (static method)
func fiberSuspend(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// Get the current fiber state from context
	stateVal := ctx.Value(fiberContextKey{})
	if stateVal == nil {
		return nil, ThrowError(ctx, FiberError, "Cannot call Fiber::suspend() when not in a Fiber")
	}
	state := stateVal.(*FiberState)

	if state.status != FiberRunning {
		return nil, ThrowError(ctx, FiberError, "Cannot suspend a fiber that is not running")
	}

	var suspendVal *phpv.ZVal
	if len(args) > 0 {
		suspendVal = args[0]
	} else {
		suspendVal = phpv.ZNULL.ZVal()
	}

	state.status = FiberSuspended

	// Send the suspend value to the caller
	state.suspendCh <- fiberMsg{val: suspendVal}

	// Wait for resume or throw
	msg := <-state.resumeCh

	state.status = FiberRunning

	if msg.err != nil {
		// The caller called $fiber->throw($exception)
		return nil, msg.err
	}

	return msg.val, nil
}

// fiberThrow implements $fiber->throw(Throwable $exception): mixed
func fiberThrow(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	state := getFiberState(o)
	if state == nil {
		return nil, ThrowError(ctx, FiberError, "Cannot throw into a fiber that is not properly initialized")
	}
	if state.status != FiberSuspended {
		return nil, ThrowError(ctx, FiberError, "Cannot throw into a fiber that is not suspended")
	}
	if len(args) < 1 {
		return nil, ThrowError(ctx, TypeError, "Fiber::throw() expects exactly 1 argument, 0 given")
	}

	// Create a PhpThrow from the exception object
	exc := args[0]
	excObj, ok := exc.Value().(phpv.ZObject)
	if !ok {
		return nil, ThrowError(ctx, TypeError, fmt.Sprintf("Fiber::throw() expects parameter 1 to be Throwable, %s given", exc.GetType()))
	}
	throwErr := &phperr.PhpThrow{Obj: excObj}

	state.status = FiberRunning

	// Send the exception to the fiber
	state.resumeCh <- fiberMsg{err: throwErr}

	// Wait for the fiber to suspend or terminate
	msg := <-state.suspendCh

	if state.status == FiberTerminated {
		if msg.err != nil {
			return nil, msg.err
		}
		return phpv.ZNULL.ZVal(), nil
	}

	return msg.val, nil
}

// fiberGetReturn implements $fiber->getReturn(): mixed
func fiberGetReturn(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	state := getFiberState(o)
	if state == nil {
		return nil, ThrowError(ctx, FiberError, "Cannot get return value of a fiber that is not properly initialized")
	}
	if state.status != FiberTerminated {
		if state.status == FiberCreated {
			return nil, ThrowError(ctx, FiberError, "Cannot get return value of a fiber that hasn't been started")
		}
		return nil, ThrowError(ctx, FiberError, "Cannot get return value of a fiber that hasn't terminated")
	}
	if state.fiberErr != nil {
		return nil, ThrowError(ctx, FiberError, "Cannot get return value of a fiber that threw an exception")
	}

	if state.returnVal == nil {
		return phpv.ZNULL.ZVal(), nil
	}
	return state.returnVal, nil
}

// fiberIsStarted implements $fiber->isStarted(): bool
func fiberIsStarted(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	state := getFiberState(o)
	if state == nil {
		return phpv.ZFalse.ZVal(), nil
	}
	return phpv.ZBool(state.status != FiberCreated).ZVal(), nil
}

// fiberIsRunning implements $fiber->isRunning(): bool
func fiberIsRunning(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	state := getFiberState(o)
	if state == nil {
		return phpv.ZFalse.ZVal(), nil
	}
	return phpv.ZBool(state.status == FiberRunning).ZVal(), nil
}

// fiberIsSuspended implements $fiber->isSuspended(): bool
func fiberIsSuspended(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	state := getFiberState(o)
	if state == nil {
		return phpv.ZFalse.ZVal(), nil
	}
	return phpv.ZBool(state.status == FiberSuspended).ZVal(), nil
}

// fiberIsTerminated implements $fiber->isTerminated(): bool
func fiberIsTerminated(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	state := getFiberState(o)
	if state == nil {
		return phpv.ZFalse.ZVal(), nil
	}
	return phpv.ZBool(state.status == FiberTerminated).ZVal(), nil
}

// fiberGetCurrent implements Fiber::getCurrent(): ?Fiber (static method)
func fiberGetCurrent(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	objVal := ctx.Value(fiberObjectKey{})
	if objVal == nil {
		return phpv.ZNULL.ZVal(), nil
	}
	obj, ok := objVal.(*ZObject)
	if !ok {
		return phpv.ZNULL.ZVal(), nil
	}
	return obj.ZVal(), nil
}
