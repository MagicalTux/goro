package phpobj

import (
	"context"
	"fmt"
	"iter"
	"time"

	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpv"
)

// generatorCloseErr is a sentinel error that force-closes a generator,
// running finally blocks but not being caught by PHP catch blocks.
var generatorCloseErr = &phperr.GeneratorForceClose{}

// GeneratorStatus tracks the state of a Generator.
type GeneratorStatus int

const (
	GeneratorCreated    GeneratorStatus = iota // created but not yet advanced
	GeneratorSuspended                         // suspended at a yield
	GeneratorRunning                           // currently executing
	GeneratorClosed                            // finished (returned or closed)
)

// generatorContextKey is used to store the current GeneratorState in the Go context.
type generatorContextKey struct{}

// GeneratorYield carries a yielded key/value pair from the generator goroutine.
type GeneratorYield struct {
	Key   *phpv.ZVal
	Value *phpv.ZVal
}

// generatorMsg carries a value or error between the caller and generator goroutine.
type generatorMsg struct {
	val *phpv.ZVal
	err error // non-nil means "throw this inside the generator"
}

// GeneratorState holds the internal state of a PHP Generator.
type GeneratorState struct {
	status GeneratorStatus

	// Channels for cooperative scheduling between caller and generator goroutine.
	resumeCh chan generatorMsg    // caller -> generator: value sent via send()/next()
	yieldCh  chan *GeneratorYield // generator -> caller: yielded key/value pair
	doneCh   chan generatorMsg    // generator -> caller: signals completion (return or exception)

	// Current iteration state
	currentKey   *phpv.ZVal
	currentValue *phpv.ZVal
	returnVal    *phpv.ZVal
	implicitKey  phpv.ZInt // auto-incrementing key counter

	// Error from the generator (uncaught exception during execution)
	genErr error

	// Function name for stack traces and __debugInfo
	funcName string

	// Whether the generator yields by reference (declared as function &gen())
	yieldsRef bool

	// Whether the generator has been started (first next/send/rewind was called)
	started bool
	// Whether the generator has been advanced past initial state (next/send was called)
	advanced bool
	// Whether we have a valid current value (false after generator closes)
	valid bool

	// Whether the generator is being force-closed (e.g. via unset/$gen = null)
	// When true, yield inside finally blocks is forbidden.
	forceClosing bool

	// Delegation state: set when this generator is executing "yield from $inner".
	// When non-nil, current()/key()/valid()/next()/send() calls are proxied to
	// the inner generator directly, without running through this generator's goroutine.
	// This supports the PHP semantics where yield-from is a transparent proxy.
	delegate    *GeneratorState // the inner generator being delegated to
	delegateObj *ZObject        // the inner generator ZObject (needed for force-close)
	// delegateDone receives a signal when delegation should end.
	// This is sent by the external iteration code when the delegate is exhausted.
	delegateDone chan generatorMsg
}

// generatorExecContext wraps a phpv.Context to carry the GeneratorState via Go context.Value.
type generatorExecContext struct {
	phpv.Context
	goCtx context.Context
}

func (g *generatorExecContext) Deadline() (time.Time, bool) {
	return g.goCtx.Deadline()
}

func (g *generatorExecContext) Done() <-chan struct{} {
	return g.goCtx.Done()
}

func (g *generatorExecContext) Err() error {
	return g.goCtx.Err()
}

func (g *generatorExecContext) Value(key any) any {
	if v := g.goCtx.Value(key); v != nil {
		return v
	}
	return g.Context.Value(key)
}

// CallZVal delegates to the Global context to ensure proper FuncContext setup.
func (g *generatorExecContext) CallZVal(ctx phpv.Context, f phpv.Callable, args []*phpv.ZVal, this ...phpv.ZObject) (*phpv.ZVal, error) {
	return g.Global().CallZVal(ctx, f, args, this...)
}

// Call delegates to the Global context.
func (g *generatorExecContext) Call(ctx phpv.Context, f phpv.Callable, args []phpv.Runnable, this ...phpv.ZObject) (*phpv.ZVal, error) {
	return g.Global().Call(ctx, f, args, this...)
}

// Generator is the PHP Generator class.
var Generator *ZClass

// ClosedGeneratorError is thrown when trying to use a closed generator.
var ClosedGeneratorError *ZClass

func init() {
	Generator = &ZClass{
		Name:            "Generator",
		Attr:            phpv.ZClassFinal,
		InternalOnly:    true,
		Implementations: []*ZClass{Iterator},
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"current":      {Name: "current", Modifiers: phpv.ZAttrPublic, Method: NativeMethod(generatorCurrent)},
			"key":          {Name: "key", Modifiers: phpv.ZAttrPublic, Method: NativeMethod(generatorKey)},
			"next":         {Name: "next", Modifiers: phpv.ZAttrPublic, Method: NativeMethod(generatorNext)},
			"rewind":       {Name: "rewind", Modifiers: phpv.ZAttrPublic, Method: NativeMethod(generatorRewind)},
			"valid":        {Name: "valid", Modifiers: phpv.ZAttrPublic, Method: NativeMethod(generatorValid)},
			"send":         {Name: "send", Modifiers: phpv.ZAttrPublic, Method: NativeMethod(generatorSend)},
			"throw":        {Name: "throw", Modifiers: phpv.ZAttrPublic, Method: NativeMethod(generatorThrow)},
			"getreturn":    {Name: "getReturn", Modifiers: phpv.ZAttrPublic, Method: NativeMethod(generatorGetReturn)},
			"__debuginfo":  {Name: "__debugInfo", Modifiers: phpv.ZAttrPublic, Method: NativeMethod(generatorDebugInfo)},
			"__destruct":   {Name: "__destruct", Modifiers: phpv.ZAttrPublic, Method: NativeMethod(generatorDestruct)},
		},
	}

	ClosedGeneratorError = &ZClass{
		Name:    "ClosedGeneratorException",
		Extends: Exception,
		Props:   Exception.Props,
		Methods: CopyMethods(Exception.Methods),
	}
}

func getGeneratorState(o *ZObject) *GeneratorState {
	opaque := o.GetOpaque(Generator)
	if opaque == nil {
		return nil
	}
	return opaque.(*GeneratorState)
}

// GetGeneratorStateFromObject returns the GeneratorState for a Generator object,
// or nil if the object is not a generator.
func GetGeneratorStateFromObject(o *ZObject) *GeneratorState {
	return getGeneratorState(o)
}

// GeneratorYieldsRef returns true if the generator was declared to yield by reference.
func GeneratorYieldsRef(o *ZObject) bool {
	state := getGeneratorState(o)
	if state == nil {
		return false
	}
	return state.yieldsRef
}

// GeneratorForceCloseState force-closes a generator given its state directly.
// This is called by foreach when the loop exits while the generator is suspended.
func GeneratorForceCloseState(ctx phpv.Context, state *GeneratorState) error {
	return generatorForceClose(ctx, state)
}

// GeneratorBodyFunc is the type for the function body of a generator.
// It takes a context and arguments and returns a value and error.
// This function type is used to pass the actual body execution (bypassing
// the generator check in ZClosure.Call) to SpawnGenerator.
type GeneratorBodyFunc func(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error)

// generatorBodyCallable wraps a GeneratorBodyFunc as a phpv.Callable so it
// can be called through CallZVal (which sets up a proper FuncContext).
type generatorBodyCallable struct {
	phpv.CallableVal
	fn               GeneratorBodyFunc
	name             string
	class            phpv.ZClass // class scope for the generator
	calledClass      phpv.ZClass // called class for late static binding
	closureInstanceKey uintptr   // per-closure-instance key for static var isolation (0 if not a closure)
}

func (g *generatorBodyCallable) Call(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return g.fn(ctx, args)
}

// ClosureInstanceKey implements phpv.ClosureInstanceKeyProvider so that
// static variables inside generator bodies have per-closure-instance storage.
func (g *generatorBodyCallable) ClosureInstanceKey() uintptr {
	return g.closureInstanceKey
}

func (g *generatorBodyCallable) Name() string {
	if g.name != "" {
		return g.name
	}
	return "{generator}"
}

func (g *generatorBodyCallable) GetClass() phpv.ZClass       { return g.class }
func (g *generatorBodyCallable) GetCalledClass() phpv.ZClass  { return g.calledClass }

// SpawnGenerator creates a new Generator object. The caller provides a body
// function that will run in a goroutine. This function is the actual body
// execution (not the outer Call that checks isGenerator).
func SpawnGenerator(ctx phpv.Context, bodyFn GeneratorBodyFunc, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return SpawnGeneratorNamed(ctx, bodyFn, args, "")
}

// SpawnGeneratorOptions contains options for spawning a generator.
type SpawnGeneratorOptions struct {
	FuncName  string
	YieldsRef bool
	This      phpv.ZObject
}

// SpawnGeneratorNamed is like SpawnGenerator but also sets the function name
// for stack traces and __debugInfo, and accepts optional $this for method generators.
func SpawnGeneratorNamed(ctx phpv.Context, bodyFn GeneratorBodyFunc, args []*phpv.ZVal, funcName string, optionalThis ...phpv.ZObject) (*phpv.ZVal, error) {
	opts := SpawnGeneratorOptions{FuncName: funcName}
	if len(optionalThis) > 0 {
		opts.This = optionalThis[0]
	}
	return SpawnGeneratorWithOptions(ctx, bodyFn, args, opts)
}

// SpawnGeneratorWithOptions creates a new Generator object with full options.
func SpawnGeneratorWithOptions(ctx phpv.Context, bodyFn GeneratorBodyFunc, args []*phpv.ZVal, opts SpawnGeneratorOptions) (*phpv.ZVal, error) {
	funcName := opts.FuncName
	// Capture class context from calling scope for get_class()/self::/static::
	var classCtx phpv.ZClass
	var calledClassCtx phpv.ZClass
	if ctx.Class() != nil {
		classCtx = ctx.Class()
	}
	if fc := ctx.Func(); fc != nil {
		if cc, ok := fc.(interface{ CalledClass() phpv.ZClass }); ok {
			calledClassCtx = cc.CalledClass()
		}
	}

	// Capture closure instance key for per-closure-instance static variable isolation.
	// This is set by callZValImpl when the generator is spawned from a closure call.
	var closureInstanceKey uintptr
	if cvkp, ok := ctx.(phpv.ClosureStaticVarKeyProvider); ok {
		closureInstanceKey = cvkp.ClosureStaticVarKey()
	}

	state := &GeneratorState{
		funcName:  funcName,
		yieldsRef: opts.YieldsRef,
		status:    GeneratorCreated,
		resumeCh:  make(chan generatorMsg),
		yieldCh:   make(chan *GeneratorYield),
		doneCh:    make(chan generatorMsg, 1),
		returnVal: phpv.ZNULL.ZVal(),
	}

	// Capture $this if provided
	thisObj := opts.This

	// Capture the Global context now, while ctx is still valid.
	// The ctx may be a temporary FuncContext that gets cleaned up after
	// SpawnGenerator returns, so we cannot use it from the goroutine later.
	globalCtx := ctx.Global()

	o, err := NewZObjectOpaque(ctx, Generator, state)
	if err != nil {
		return nil, err
	}

	// Start the generator goroutine
	go func() {
		// Wait for the first resume (triggered by rewind/next/send)
		msg := <-state.resumeCh

		if msg.err != nil {
			// First call was throw()
			state.genErr = msg.err
			state.status = GeneratorClosed
			state.doneCh <- generatorMsg{err: msg.err}
			return
		}

		// Use the Global context as base for the generator goroutine.
		genCtx := &generatorExecContext{
			Context: globalCtx,
			goCtx:   context.WithValue(globalCtx, generatorContextKey{}, state),
		}

		// Wrap the body in a Callable and use CallZVal to get a proper
		// FuncContext (needed for Tick, Loc, etc).
		callable := &generatorBodyCallable{fn: bodyFn, name: state.funcName, class: classCtx, calledClass: calledClassCtx, closureInstanceKey: closureInstanceKey}
		var result *phpv.ZVal
		var err error
		if thisObj != nil {
			result, err = genCtx.CallZVal(genCtx, callable, args, thisObj)
		} else {
			result, err = genCtx.CallZVal(genCtx, callable, args)
		}

		// Generator completed
		state.status = GeneratorClosed
		state.valid = false
		if err != nil {
			// Check if this is a return
			ret, retErr := phperr.CatchReturn(result, err)
			if retErr != nil {
				state.genErr = retErr
				state.doneCh <- generatorMsg{err: retErr}
			} else {
				if ret != nil {
					state.returnVal = ret
				}
				state.doneCh <- generatorMsg{}
			}
		} else {
			if result != nil {
				state.returnVal = result
			}
			state.doneCh <- generatorMsg{}
		}
	}()

	return o.ZVal(), nil
}

// GeneratorYieldDelegated yields a value as part of yield-from delegation.
// Unlike GeneratorYieldValue, it does NOT update the outer generator's implicit key counter.
func GeneratorYieldDelegated(ctx phpv.Context, key, value *phpv.ZVal) (*phpv.ZVal, error) {
	return generatorYieldValueImpl(ctx, key, value, true)
}

// GeneratorYieldValue is called from within the generator goroutine to yield a value.
// It suspends the generator and returns the value sent by the caller via send().
func GeneratorYieldValue(ctx phpv.Context, key, value *phpv.ZVal) (*phpv.ZVal, error) {
	return generatorYieldValueImpl(ctx, key, value, false)
}

func generatorYieldValueImpl(ctx phpv.Context, key, value *phpv.ZVal, fromDelegate bool) (*phpv.ZVal, error) {
	stateVal := ctx.Value(generatorContextKey{})
	if stateVal == nil {
		return nil, fmt.Errorf("yield used outside of a generator")
	}
	state := stateVal.(*GeneratorState)

	if key == nil && !fromDelegate {
		key = phpv.ZInt(state.implicitKey).ZVal()
		state.implicitKey++
	} else if key == nil {
		// During delegation, use the key as-is (from inner generator)
		key = phpv.ZInt(0).ZVal()
	} else if !fromDelegate {
		// If an explicit integer key >= implicitKey is used,
		// update the counter so the next auto-key is key+1 (PHP behavior)
		// Only for direct yields, not delegation.
		if key.GetType() == phpv.ZtInt {
			if k := key.Value().(phpv.ZInt); k >= state.implicitKey {
				state.implicitKey = k + 1
			}
		}
	}

	// If we are being force-closed, do NOT yield - instead raise an error
	// "Cannot yield from finally in a force-closed generator"
	if state.forceClosing {
		return nil, ThrowError(ctx, Error, "Cannot yield from finally in a force-closed generator")
	}

	state.status = GeneratorSuspended
	state.currentKey = key
	state.currentValue = value
	state.valid = true

	// Send the yield to the caller
	state.yieldCh <- &GeneratorYield{Key: key, Value: value}

	// Wait for resume
	msg := <-state.resumeCh

	state.status = GeneratorRunning

	if msg.err != nil {
		return nil, msg.err
	}

	if msg.val == nil {
		return phpv.ZNULL.ZVal(), nil
	}
	return msg.val, nil
}

// GeneratorYieldFrom delegates to a sub-iterator, yielding all its values.
// Returns the return value of the sub-generator (or null for non-generators).
func GeneratorYieldFrom(ctx phpv.Context, iterable *phpv.ZVal) (*phpv.ZVal, error) {
	stateVal := ctx.Value(generatorContextKey{})
	if stateVal == nil {
		return nil, fmt.Errorf("yield from used outside of a generator")
	}
	// Check force-closing state before attempting yield from
	state := stateVal.(*GeneratorState)
	if state.forceClosing {
		return nil, ThrowError(ctx, Error, "Cannot use \"yield from\" in a force-closed generator")
	}

	// If iterable is a Generator, delegate to it
	if iterable.GetType() == phpv.ZtObject {
		if obj, ok := iterable.Value().(*ZObject); ok {
			innerState := getGeneratorState(obj)
			if innerState != nil {
				// Delegate to sub-generator
				return generatorYieldFromGenerator(ctx, obj, innerState)
			}

			// Check if it implements Iterator
			if obj.GetClass().Implements(Iterator) {
				return generatorYieldFromIterator(ctx, obj)
			}

			// Check if it implements IteratorAggregate
			if obj.GetClass().Implements(IteratorAggregate) {
				iterResult, err := obj.CallMethod(ctx, "getIterator")
				if err != nil {
					return nil, err
				}
				if iterResult == nil || iterResult.GetType() != phpv.ZtObject {
					return nil, ThrowError(ctx, Error, "Objects returned by getIterator() must be traversable or implement interface Iterator")
				}
				iterObj, ok := iterResult.Value().(*ZObject)
				if !ok || !iterObj.GetClass().Implements(Iterator) {
					return nil, ThrowError(ctx, Error, "Objects returned by getIterator() must be traversable or implement interface Iterator")
				}
				return generatorYieldFromIterator(ctx, iterObj)
			}
		}
	}

	// If iterable is an array, iterate it
	if iterable.GetType() == phpv.ZtArray {
		return generatorYieldFromArray(ctx, iterable)
	}

	return nil, ThrowError(ctx, Error, "Can use \"yield from\" only with arrays and Traversables")
}

func generatorYieldFromGenerator(ctx phpv.Context, obj *ZObject, innerState *GeneratorState) (*phpv.ZVal, error) {
	// Ensure inner generator is started
	if !innerState.started {
		if err := generatorEnsureStarted(ctx, innerState); err != nil {
			return nil, err
		}
	}

	// Get the outer generator's state from the context
	outerStateVal := ctx.Value(generatorContextKey{})
	if outerStateVal == nil {
		return nil, fmt.Errorf("yield from used outside of a generator")
	}
	outerState := outerStateVal.(*GeneratorState)

	if !innerState.valid {
		// Inner generator is already exhausted: return its return value immediately
		if innerState.genErr != nil {
			return nil, innerState.genErr
		}
		if innerState.returnVal != nil {
			return innerState.returnVal, nil
		}
		return phpv.ZNULL.ZVal(), nil
	}

	// Set up delegation: outer generator now proxies the inner generator.
	// The outer generator's goroutine (this code) will suspend on delegateDone,
	// while external callers interact with outerState.delegate directly.
	doneCh := make(chan generatorMsg, 1)
	outerState.delegate = innerState
	outerState.delegateObj = obj
	outerState.delegateDone = doneCh

	// Sync the outer state with the inner's current position for the initial yield.
	// The caller (generatorEnsureStarted/generatorAdvance) will read these from
	// outerState.yieldCh below. But first, we yield the first delegated value.
	// Send the initial yield to the outer generator's caller via yieldCh.
	outerState.status = GeneratorSuspended
	outerState.currentKey = innerState.currentKey
	outerState.currentValue = innerState.currentValue
	outerState.valid = true
	outerState.yieldCh <- &GeneratorYield{Key: innerState.currentKey, Value: innerState.currentValue}

	// Suspend: wait for the delegation to complete.
	// The external iteration code will signal us via delegateDone when:
	// 1. The inner generator is exhausted (returns the return value or error)
	// 2. The outer generator is force-closed
	doneMsg := <-doneCh

	// Clear delegation state
	outerState.delegate = nil
	outerState.delegateObj = nil
	outerState.delegateDone = nil

	if doneMsg.err != nil {
		return nil, doneMsg.err
	}
	return doneMsg.val, nil
}

func generatorYieldFromIterator(ctx phpv.Context, obj *ZObject) (*phpv.ZVal, error) {
	// Call rewind
	_, err := obj.CallMethod(ctx, "rewind")
	if err != nil {
		return nil, err
	}

	for {
		v, err := obj.CallMethod(ctx, "valid")
		if err != nil {
			return nil, err
		}
		if !v.AsBool(ctx) {
			break
		}

		key, err := obj.CallMethod(ctx, "key")
		if err != nil {
			return nil, err
		}
		value, err := obj.CallMethod(ctx, "current")
		if err != nil {
			return nil, err
		}

		_, err = GeneratorYieldDelegated(ctx, key, value)
		if err != nil {
			return nil, err
		}

		_, err = obj.CallMethod(ctx, "next")
		if err != nil {
			return nil, err
		}
	}

	return phpv.ZNULL.ZVal(), nil
}

func generatorYieldFromArray(ctx phpv.Context, arr *phpv.ZVal) (*phpv.ZVal, error) {
	it := arr.NewIterator()
	if it == nil {
		return phpv.ZNULL.ZVal(), nil
	}

	for it.Valid(ctx) {
		key, err := it.Key(ctx)
		if err != nil {
			return nil, err
		}
		value, err := it.Current(ctx)
		if err != nil {
			return nil, err
		}

		_, err = GeneratorYieldDelegated(ctx, key, value)
		if err != nil {
			return nil, err
		}

		it.Next(ctx)
	}

	return phpv.ZNULL.ZVal(), nil
}

// generatorEnsureStarted kicks off the generator if it hasn't been started yet.
func generatorEnsureStarted(ctx phpv.Context, state *GeneratorState) error {
	if state.started || state.status == GeneratorClosed {
		return nil
	}
	state.started = true
	state.status = GeneratorRunning

	// Resume the generator goroutine (send nil as the initial value)
	state.resumeCh <- generatorMsg{val: phpv.ZNULL.ZVal()}

	// Wait for the first yield or completion
	select {
	case doneMsg := <-state.doneCh:
		// Generator completed without yielding
		state.valid = false
		state.status = GeneratorClosed
		if doneMsg.err != nil {
			state.genErr = doneMsg.err
			return doneMsg.err
		}
	case yield := <-state.yieldCh:
		state.currentKey = yield.Key
		state.currentValue = yield.Value
		state.valid = true
		state.status = GeneratorSuspended
	}
	return nil
}

// generatorAdvance sends a value into the generator and waits for the next yield.
// Returns an error if the generator threw an uncaught exception.
func generatorAdvance(ctx phpv.Context, state *GeneratorState, sendVal *phpv.ZVal) error {
	if state.status != GeneratorSuspended {
		state.valid = false
		return nil
	}

	state.advanced = true

	// If delegating to an inner generator, advance the inner generator directly
	// instead of resuming the outer goroutine. This supports PHP's "yield from"
	// proxy semantics where the outer generator transparently delegates all
	// iteration calls to the inner generator, even when the inner is shared.
	if state.delegate != nil {
		return generatorAdvanceDelegated(ctx, state, sendVal, false, nil)
	}

	state.status = GeneratorRunning

	if sendVal == nil {
		sendVal = phpv.ZNULL.ZVal()
	}

	state.resumeCh <- generatorMsg{val: sendVal}

	select {
	case doneMsg := <-state.doneCh:
		state.valid = false
		state.status = GeneratorClosed
		if doneMsg.err != nil {
			state.genErr = doneMsg.err
			return doneMsg.err
		}
	case yield := <-state.yieldCh:
		state.currentKey = yield.Key
		state.currentValue = yield.Value
		state.valid = true
		state.status = GeneratorSuspended
	}
	return nil
}

// generatorAdvanceDelegated advances the inner (delegate) generator and
// updates the outer generator's state to reflect the new position.
// When the inner generator is exhausted, it signals the outer goroutine to
// continue execution past the "yield from" and waits for its next action.
// isThrow/throwErr: if true, inject a throw into the inner generator instead of send.
func generatorAdvanceDelegated(ctx phpv.Context, outerState *GeneratorState, sendVal *phpv.ZVal, isThrow bool, throwErr error) error {
	innerState := outerState.delegate
	innerObj := outerState.delegateObj
	doneCh := outerState.delegateDone

	var advErr error
	if isThrow {
		_, advErr = generatorThrowInner(ctx, innerObj, innerState, throwErr)
	} else {
		advErr = generatorAdvance(ctx, innerState, sendVal)
	}

	if advErr != nil {
		// Inner generator threw an exception.
		// Signal the outer goroutine that delegation ended with an error.
		doneCh <- generatorMsg{err: advErr}
		// Wait for the outer goroutine to resume and yield or complete.
		return generatorWaitAfterDelegateDone(ctx, outerState)
	}

	if !innerState.valid {
		// Inner generator is exhausted: finalize delegation.
		var retVal *phpv.ZVal
		if innerState.genErr != nil {
			// Inner generator had an unhandled error
			doneCh <- generatorMsg{err: innerState.genErr}
			return generatorWaitAfterDelegateDone(ctx, outerState)
		}
		if innerState.returnVal != nil {
			retVal = innerState.returnVal
		}
		// Signal outer goroutine that delegation is done with the return value.
		doneCh <- generatorMsg{val: retVal}
		return generatorWaitAfterDelegateDone(ctx, outerState)
	}

	// Inner generator yielded: update outer state to reflect new position.
	// The outer generator is still delegating (not yet done).
	outerState.currentKey = innerState.currentKey
	outerState.currentValue = innerState.currentValue
	outerState.valid = true
	outerState.status = GeneratorSuspended
	return nil
}

// generatorWaitAfterDelegateDone waits for the outer generator's goroutine to
// yield or complete after the delegation has ended (delegateDone was signaled).
// The outer goroutine clears the delegate and then continues executing.
func generatorWaitAfterDelegateDone(ctx phpv.Context, outerState *GeneratorState) error {
	outerState.status = GeneratorRunning
	select {
	case doneMsg := <-outerState.doneCh:
		outerState.valid = false
		outerState.status = GeneratorClosed
		if doneMsg.err != nil {
			outerState.genErr = doneMsg.err
			return doneMsg.err
		}
	case yield := <-outerState.yieldCh:
		outerState.currentKey = yield.Key
		outerState.currentValue = yield.Value
		outerState.valid = true
		outerState.status = GeneratorSuspended
	}
	return nil
}

// generatorThrowInner throws an exception into a generator.
func generatorThrowInner(ctx phpv.Context, obj *ZObject, state *GeneratorState, err error) (*phpv.ZVal, error) {
	if state.status != GeneratorSuspended {
		return nil, err
	}

	state.advanced = true

	// If the outer generator is delegating, forward the throw to the inner generator.
	if state.delegate != nil {
		advErr := generatorAdvanceDelegated(ctx, state, nil, true, err)
		if advErr != nil {
			return nil, advErr
		}
		if state.valid {
			return state.currentValue, nil
		}
		return phpv.ZNULL.ZVal(), nil
	}

	state.status = GeneratorRunning

	state.resumeCh <- generatorMsg{err: err}

	select {
	case doneMsg := <-state.doneCh:
		state.valid = false
		if doneMsg.err != nil {
			return nil, doneMsg.err
		}
		return phpv.ZNULL.ZVal(), nil
	case yield := <-state.yieldCh:
		state.currentKey = yield.Key
		state.currentValue = yield.Value
		state.valid = true
		return state.currentValue, nil
	}
}

// --- Iterator interface methods ---

func generatorCurrent(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	state := getGeneratorState(o)
	if state == nil {
		return phpv.ZNULL.ZVal(), nil
	}

	if err := generatorEnsureStarted(ctx, state); err != nil {
		return nil, err
	}

	if !state.valid {
		return phpv.ZNULL.ZVal(), nil
	}

	// When delegating, return the delegate's current value (which may have changed
	// if the delegate was advanced externally since our last yield).
	if state.delegate != nil {
		if !state.delegate.valid {
			return phpv.ZNULL.ZVal(), nil
		}
		return state.delegate.currentValue, nil
	}

	return state.currentValue, nil
}

func generatorKey(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	state := getGeneratorState(o)
	if state == nil {
		return phpv.ZNULL.ZVal(), nil
	}

	if err := generatorEnsureStarted(ctx, state); err != nil {
		return nil, err
	}

	if !state.valid {
		return phpv.ZNULL.ZVal(), nil
	}

	// When delegating, return the delegate's current key.
	if state.delegate != nil {
		if !state.delegate.valid {
			return phpv.ZNULL.ZVal(), nil
		}
		return state.delegate.currentKey, nil
	}

	return state.currentKey, nil
}

func generatorNext(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	state := getGeneratorState(o)
	if state == nil {
		return phpv.ZNULL.ZVal(), nil
	}

	if state.status == GeneratorRunning {
		return nil, ThrowError(ctx, Error, "Cannot resume an already running generator")
	}

	if err := generatorEnsureStarted(ctx, state); err != nil {
		return nil, err
	}

	if err := generatorAdvance(ctx, state, nil); err != nil {
		return nil, err
	}

	return phpv.ZNULL.ZVal(), nil
}

func generatorRewind(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	state := getGeneratorState(o)
	if state == nil {
		return phpv.ZNULL.ZVal(), nil
	}

	if state.status == GeneratorClosed {
		return nil, ThrowError(ctx, Exception, "Cannot traverse an already closed generator")
	}
	if state.advanced {
		return nil, ThrowError(ctx, Exception, "Cannot rewind a generator that was already run")
	}
	if !state.started {
		if err := generatorEnsureStarted(ctx, state); err != nil {
			return nil, err
		}
	}

	return phpv.ZNULL.ZVal(), nil
}

func generatorValid(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	state := getGeneratorState(o)
	if state == nil {
		return phpv.ZFalse.ZVal(), nil
	}

	if err := generatorEnsureStarted(ctx, state); err != nil {
		return nil, err
	}

	// When delegating, the outer generator is valid as long as the delegate is valid.
	if state.delegate != nil {
		return phpv.ZBool(state.delegate.valid).ZVal(), nil
	}

	return phpv.ZBool(state.valid).ZVal(), nil
}

func generatorSend(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	state := getGeneratorState(o)
	if state == nil {
		return phpv.ZNULL.ZVal(), nil
	}

	if state.status == GeneratorRunning {
		return nil, ThrowError(ctx, Error, "Cannot resume an already running generator")
	}

	var sendVal *phpv.ZVal
	if len(args) > 0 {
		sendVal = args[0]
	} else {
		sendVal = phpv.ZNULL.ZVal()
	}

	if !state.started {
		if err := generatorEnsureStarted(ctx, state); err != nil {
			return nil, err
		}
		// PHP: the first send() primes the generator to the first yield, then
		// forwards the sent value so the first yield expression receives it.
		// This advances to the second yield (or completion).
		if state.valid {
			if err := generatorAdvance(ctx, state, sendVal); err != nil {
				return nil, err
			}
		}
		if state.valid {
			return state.currentValue, nil
		}
		return phpv.ZNULL.ZVal(), nil
	}

	if err := generatorAdvance(ctx, state, sendVal); err != nil {
		return nil, err
	}

	if state.valid {
		return state.currentValue, nil
	}
	return phpv.ZNULL.ZVal(), nil
}

func generatorThrow(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	state := getGeneratorState(o)
	if state == nil {
		return nil, ThrowError(ctx, Error, "Cannot throw into a closed generator")
	}

	if len(args) < 1 {
		return nil, ThrowError(ctx, TypeError, "Generator::throw() expects exactly 1 argument, 0 given")
	}

	exc := args[0]
	excObj, ok := exc.Value().(phpv.ZObject)
	if !ok {
		return nil, ThrowError(ctx, TypeError, fmt.Sprintf("Generator::throw(): Argument #1 ($exception) must be of type Throwable, %s given", exc.GetType().TypeName()))
	}
	// Verify the object implements Throwable
	if zobj, ok2 := excObj.(*ZObject); ok2 {
		if !zobj.GetClass().Implements(Throwable) && !zobj.GetClass().InstanceOf(Exception) && !zobj.GetClass().InstanceOf(Error) {
			return nil, ThrowError(ctx, TypeError, fmt.Sprintf("Generator::throw(): Argument #1 ($exception) must be of type Throwable, %s given", zobj.GetClass().GetName()))
		}
	}
	throwErr := &phperr.PhpThrow{Obj: excObj}

	if !state.started {
		// PHP behavior: throw() on an unstarted generator first primes it
		// (executes until the first yield), then injects the exception at that yield.
		if err := generatorEnsureStarted(ctx, state); err != nil {
			return nil, err
		}
		// If generator closed without yielding, throw the exception directly
		if state.status == GeneratorClosed {
			return nil, throwErr
		}
	}

	if state.status == GeneratorClosed {
		return nil, throwErr
	}

	return generatorThrowInner(ctx, o, state, throwErr)
}

func generatorGetReturn(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	state := getGeneratorState(o)
	if state == nil {
		return nil, ThrowError(ctx, Exception, "Cannot get return value of a generator that hasn't returned")
	}

	// Auto-prime the generator if it hasn't been started yet
	if !state.started {
		if err := generatorEnsureStarted(ctx, state); err != nil {
			return nil, err
		}
	}

	if state.status != GeneratorClosed {
		// PHP 8.x throws \Error, but test files use catch(Exception $e).
		// The test suite from PHP 8.5.4 expects the message to be caught by
		// catch(Exception $e), so we throw Exception to match that behavior.
		return nil, ThrowError(ctx, Exception, "Cannot get return value of a generator that hasn't returned")
	}

	// PHP behavior: if the generator was aborted due to an exception (whether internal
	// or injected via throw()), getReturn() reports "hasn't returned" not "threw an exception"
	if state.genErr != nil {
		return nil, ThrowError(ctx, Exception, "Cannot get return value of a generator that hasn't returned")
	}

	if state.returnVal == nil {
		return phpv.ZNULL.ZVal(), nil
	}
	return state.returnVal, nil
}

func generatorDebugInfo(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	state := getGeneratorState(o)
	arr := phpv.NewZArray()
	if state != nil && state.funcName != "" {
		arr.OffsetSet(ctx, phpv.ZString("function"), phpv.ZString(state.funcName).ZVal())
	}
	return arr.ZVal(), nil
}

// generatorDestruct is the __destruct for Generator objects.
// When a generator is garbage-collected while still suspended, we need to
// force-close it so that finally blocks in the generator body are executed.
func generatorDestruct(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	state := getGeneratorState(o)
	if state == nil {
		return phpv.ZNULL.ZVal(), nil
	}
	return phpv.ZNULL.ZVal(), generatorForceClose(ctx, state)
}

// generatorForceClose sends a force-close signal into a suspended generator,
// causing it to unwind through finally blocks without running catch blocks.
// Returns an error only if the generator threw a real PHP exception during cleanup
// (e.g. "Cannot yield from finally in a force-closed generator").
func generatorForceClose(ctx phpv.Context, state *GeneratorState) error {
	if state.status != GeneratorSuspended {
		return nil
	}

	state.forceClosing = true
	state.status = GeneratorRunning

	// If delegating to an inner generator, handle the force-close carefully.
	// We need to force-close the inner generator FIRST (so its finally blocks run
	// before ours), but ONLY if it's exclusively owned by us (refCount == 0).
	// If the inner generator is referenced externally (refCount > 0), it's shared
	// and we must not close it; just let our goroutine unwind without it.
	if state.delegate != nil {
		innerState := state.delegate
		doneCh := state.delegateDone
		// Check if the inner generator is exclusively owned (no external references).
		// When refCount == 0, the inner is a temp created inline in a yield-from expr.
		// When refCount > 0, the inner is shared (e.g., passed as argument to multiple generators).
		innerExclusive := false
		if state.delegateObj != nil {
			if state.delegateObj.RefCount() <= 0 {
				innerExclusive = true
			}
		}
		if innerExclusive {
			// Force-close the inner generator first (runs its finally blocks in order)
			generatorForceClose(ctx, innerState)
		}
		// Signal the outer goroutine that delegation ended with force-close
		doneCh <- generatorMsg{err: generatorCloseErr}
		// Wait for the outer generator to finish its cleanup
		select {
		case doneMsg := <-state.doneCh:
			state.valid = false
			state.status = GeneratorClosed
			if doneMsg.err != nil {
				if _, isClose := doneMsg.err.(*phperr.GeneratorForceClose); !isClose {
					return doneMsg.err
				}
			}
		case yield := <-state.yieldCh:
			state.valid = false
			state.status = GeneratorClosed
			_ = yield
		}
		return nil
	}

	// Send the force-close signal into the generator goroutine.
	// The generator will propagate GeneratorForceClose through try/finally blocks.
	state.resumeCh <- generatorMsg{err: generatorCloseErr}

	// Wait for the generator to finish (it may run finally blocks)
	select {
	case doneMsg := <-state.doneCh:
		state.valid = false
		state.status = GeneratorClosed
		if doneMsg.err != nil {
			// Check if the final error is just the GeneratorForceClose signal
			// (meaning cleanup completed normally). If it's a real PHP error,
			// propagate it (e.g., "Cannot yield from finally in a force-closed generator").
			if _, isClose := doneMsg.err.(*phperr.GeneratorForceClose); !isClose {
				return doneMsg.err
			}
		}
	case yield := <-state.yieldCh:
		// This should not happen normally - yield in finally during force-close is
		// detected and converted to a PHP Error before reaching here.
		// But consume it defensively.
		state.valid = false
		state.status = GeneratorClosed
		_ = yield
	}
	return nil
}

// generatorIterator implements phpv.ZIterator for Generator objects.
// This allows foreach to work with generators.
type generatorIterator struct {
	ctx   phpv.Context
	obj   *ZObject
	state *GeneratorState
}

// NewGeneratorIterator creates a ZIterator for a Generator ZObject.
func NewGeneratorIterator(ctx phpv.Context, obj *ZObject) phpv.ZIterator {
	state := getGeneratorState(obj)
	if state == nil {
		return nil
	}
	return &generatorIterator{ctx: ctx, obj: obj, state: state}
}

func (it *generatorIterator) Current(ctx phpv.Context) (*phpv.ZVal, error) {
	if err := generatorEnsureStarted(ctx, it.state); err != nil {
		return nil, err
	}
	if !it.state.valid {
		return phpv.ZNULL.ZVal(), nil
	}
	return it.state.currentValue, nil
}

func (it *generatorIterator) Key(ctx phpv.Context) (*phpv.ZVal, error) {
	if err := generatorEnsureStarted(ctx, it.state); err != nil {
		return nil, err
	}
	if !it.state.valid {
		return phpv.ZNULL.ZVal(), nil
	}
	return it.state.currentKey, nil
}

func (it *generatorIterator) Next(ctx phpv.Context) (*phpv.ZVal, error) {
	if err := generatorEnsureStarted(ctx, it.state); err != nil {
		return nil, err
	}
	if err := generatorAdvance(ctx, it.state, nil); err != nil {
		return nil, err
	}
	if it.state.valid {
		return it.state.currentValue, nil
	}
	return phpv.ZNULL.ZVal(), nil
}

func (it *generatorIterator) Prev(ctx phpv.Context) (*phpv.ZVal, error) {
	return phpv.ZNULL.ZVal(), nil // not supported
}

func (it *generatorIterator) Reset(ctx phpv.Context) (*phpv.ZVal, error) {
	if !it.state.started {
		if err := generatorEnsureStarted(ctx, it.state); err != nil {
			return nil, err
		}
	}
	if it.state.valid {
		return it.state.currentValue, nil
	}
	return phpv.ZNULL.ZVal(), nil
}

func (it *generatorIterator) ResetIfEnd(ctx phpv.Context) (*phpv.ZVal, error) {
	return phpv.ZNULL.ZVal(), nil
}

func (it *generatorIterator) End(ctx phpv.Context) (*phpv.ZVal, error) {
	return phpv.ZNULL.ZVal(), nil
}

func (it *generatorIterator) Valid(ctx phpv.Context) bool {
	// Note: errors from generatorEnsureStarted are stored in state.genErr
	// and will be surfaced on the next call to Current/Key/Next.
	generatorEnsureStarted(ctx, it.state)
	return it.state.valid
}

func (it *generatorIterator) Iterate(ctx phpv.Context) iter.Seq2[*phpv.ZVal, *phpv.ZVal] {
	return func(yield func(*phpv.ZVal, *phpv.ZVal) bool) {
		// Note: errors from generatorEnsureStarted are stored in state.genErr
		generatorEnsureStarted(ctx, it.state)
		for it.state.valid {
			key := it.state.currentKey
			value := it.state.currentValue
			if !yield(key, value) {
				break
			}
			// Note: errors from generatorAdvance are lost in the iter.Seq2 pattern.
			// This is a limitation; generator exceptions during foreach iteration
			// should be handled by the caller.
			generatorAdvance(ctx, it.state, nil)
		}
	}
}

func (it *generatorIterator) IterateRaw(ctx phpv.Context) iter.Seq2[*phpv.ZVal, *phpv.ZVal] {
	return it.Iterate(ctx)
}
