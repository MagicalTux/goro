package phpobj

import (
	"context"
	"fmt"
	"iter"
	"time"

	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpv"
)

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
	resumeCh  chan generatorMsg    // caller -> generator: value sent via send()/next()
	yieldCh   chan *GeneratorYield // generator -> caller: yielded key/value pair
	doneCh    chan generatorMsg    // generator -> caller: signals completion (return or exception)

	// Current iteration state
	currentKey   *phpv.ZVal
	currentValue *phpv.ZVal
	returnVal    *phpv.ZVal
	implicitKey  phpv.ZInt // auto-incrementing key counter

	// Error from the generator (uncaught exception during execution)
	genErr error

	// Whether the generator has been started (first next/send/rewind was called)
	started bool
	// Whether we have a valid current value (false after generator closes)
	valid bool
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

// Generator is the PHP Generator class.
var Generator *ZClass

// ClosedGeneratorError is thrown when trying to use a closed generator.
var ClosedGeneratorError *ZClass

func init() {
	Generator = &ZClass{
		Name:            "Generator",
		InternalOnly:    true,
		Implementations: []*ZClass{Iterator},
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"current":   {Name: "current", Modifiers: phpv.ZAttrPublic, Method: NativeMethod(generatorCurrent)},
			"key":       {Name: "key", Modifiers: phpv.ZAttrPublic, Method: NativeMethod(generatorKey)},
			"next":      {Name: "next", Modifiers: phpv.ZAttrPublic, Method: NativeMethod(generatorNext)},
			"rewind":    {Name: "rewind", Modifiers: phpv.ZAttrPublic, Method: NativeMethod(generatorRewind)},
			"valid":     {Name: "valid", Modifiers: phpv.ZAttrPublic, Method: NativeMethod(generatorValid)},
			"send":      {Name: "send", Modifiers: phpv.ZAttrPublic, Method: NativeMethod(generatorSend)},
			"throw":     {Name: "throw", Modifiers: phpv.ZAttrPublic, Method: NativeMethod(generatorThrow)},
			"getreturn": {Name: "getReturn", Modifiers: phpv.ZAttrPublic, Method: NativeMethod(generatorGetReturn)},
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

// GeneratorBodyFunc is the type for the function body of a generator.
// It takes a context and arguments and returns a value and error.
// This function type is used to pass the actual body execution (bypassing
// the generator check in ZClosure.Call) to SpawnGenerator.
type GeneratorBodyFunc func(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error)

// SpawnGenerator creates a new Generator object. The caller provides a body
// function that will run in a goroutine. This function is the actual body
// execution (not the outer Call that checks isGenerator).
func SpawnGenerator(ctx phpv.Context, bodyFn GeneratorBodyFunc, args []*phpv.ZVal) (*phpv.ZVal, error) {
	state := &GeneratorState{
		status:    GeneratorCreated,
		resumeCh:  make(chan generatorMsg),
		yieldCh:   make(chan *GeneratorYield),
		doneCh:    make(chan generatorMsg, 1),
		returnVal: phpv.ZNULL.ZVal(),
	}

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

		// Create a context that carries the generator state
		genCtx := &generatorExecContext{
			Context: ctx,
			goCtx:   context.WithValue(ctx, generatorContextKey{}, state),
		}

		// Call the body function directly (NOT through CallZVal, which would
		// re-trigger the generator check in ZClosure.Call)
		result, err := bodyFn(genCtx, args)

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

// GeneratorYieldValue is called from within the generator goroutine to yield a value.
// It suspends the generator and returns the value sent by the caller via send().
func GeneratorYieldValue(ctx phpv.Context, key, value *phpv.ZVal) (*phpv.ZVal, error) {
	stateVal := ctx.Value(generatorContextKey{})
	if stateVal == nil {
		return nil, fmt.Errorf("yield used outside of a generator")
	}
	state := stateVal.(*GeneratorState)

	if key == nil {
		key = phpv.ZInt(state.implicitKey).ZVal()
		state.implicitKey++
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

	return nil, ThrowError(ctx, Error, fmt.Sprintf("Can use \"yield from\" only with arrays and Traversables, %s given", iterable.GetType().TypeName()))
}

func generatorYieldFromGenerator(ctx phpv.Context, obj *ZObject, innerState *GeneratorState) (*phpv.ZVal, error) {
	// Ensure inner generator is started
	if !innerState.started {
		generatorEnsureStarted(ctx, innerState)
	}

	for innerState.valid {
		// Yield the inner generator's current value
		result, err := GeneratorYieldValue(ctx, innerState.currentKey, innerState.currentValue)
		if err != nil {
			// Forward throw to inner generator
			if _, ok := err.(*phperr.PhpThrow); ok {
				throwResult, throwErr := generatorThrowInner(ctx, obj, innerState, err)
				if throwErr != nil {
					return nil, throwErr
				}
				_ = throwResult
				continue
			}
			return nil, err
		}
		// Forward send value to inner generator
		generatorAdvance(ctx, innerState, result)
	}

	// Return the inner generator's return value
	if innerState.genErr != nil {
		return nil, innerState.genErr
	}
	if innerState.returnVal != nil {
		return innerState.returnVal, nil
	}
	return phpv.ZNULL.ZVal(), nil
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

		_, err = GeneratorYieldValue(ctx, key, value)
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

		_, err = GeneratorYieldValue(ctx, key, value)
		if err != nil {
			return nil, err
		}

		it.Next(ctx)
	}

	return phpv.ZNULL.ZVal(), nil
}

// generatorEnsureStarted kicks off the generator if it hasn't been started yet.
func generatorEnsureStarted(ctx phpv.Context, state *GeneratorState) {
	if state.started || state.status == GeneratorClosed {
		return
	}
	state.started = true
	state.status = GeneratorRunning

	// Resume the generator goroutine (send nil as the initial value)
	state.resumeCh <- generatorMsg{val: phpv.ZNULL.ZVal()}

	// Wait for the first yield or completion
	select {
	case <-state.doneCh:
		// Generator completed without yielding
		state.valid = false
	case yield := <-state.yieldCh:
		state.currentKey = yield.Key
		state.currentValue = yield.Value
		state.valid = true
	}
}

// generatorAdvance sends a value into the generator and waits for the next yield.
func generatorAdvance(ctx phpv.Context, state *GeneratorState, sendVal *phpv.ZVal) {
	if state.status != GeneratorSuspended {
		state.valid = false
		return
	}

	state.status = GeneratorRunning

	if sendVal == nil {
		sendVal = phpv.ZNULL.ZVal()
	}

	state.resumeCh <- generatorMsg{val: sendVal}

	select {
	case <-state.doneCh:
		state.valid = false
	case yield := <-state.yieldCh:
		state.currentKey = yield.Key
		state.currentValue = yield.Value
		state.valid = true
	}
}

// generatorThrowInner throws an exception into a generator.
func generatorThrowInner(ctx phpv.Context, obj *ZObject, state *GeneratorState, err error) (*phpv.ZVal, error) {
	if state.status != GeneratorSuspended {
		return nil, err
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

	generatorEnsureStarted(ctx, state)

	if !state.valid {
		return phpv.ZNULL.ZVal(), nil
	}

	return state.currentValue, nil
}

func generatorKey(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	state := getGeneratorState(o)
	if state == nil {
		return phpv.ZNULL.ZVal(), nil
	}

	generatorEnsureStarted(ctx, state)

	if !state.valid {
		return phpv.ZNULL.ZVal(), nil
	}

	return state.currentKey, nil
}

func generatorNext(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	state := getGeneratorState(o)
	if state == nil {
		return phpv.ZNULL.ZVal(), nil
	}

	generatorEnsureStarted(ctx, state)

	generatorAdvance(ctx, state, nil)

	return phpv.ZNULL.ZVal(), nil
}

func generatorRewind(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	state := getGeneratorState(o)
	if state == nil {
		return phpv.ZNULL.ZVal(), nil
	}

	// Rewind is only meaningful before the generator starts.
	// If already started, it's a no-op (PHP emits no error).
	if !state.started {
		generatorEnsureStarted(ctx, state)
	}

	return phpv.ZNULL.ZVal(), nil
}

func generatorValid(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	state := getGeneratorState(o)
	if state == nil {
		return phpv.ZFalse.ZVal(), nil
	}

	generatorEnsureStarted(ctx, state)

	return phpv.ZBool(state.valid).ZVal(), nil
}

func generatorSend(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	state := getGeneratorState(o)
	if state == nil {
		return phpv.ZNULL.ZVal(), nil
	}

	var sendVal *phpv.ZVal
	if len(args) > 0 {
		sendVal = args[0]
	} else {
		sendVal = phpv.ZNULL.ZVal()
	}

	if !state.started {
		generatorEnsureStarted(ctx, state)
		// For the first send(), PHP ignores the sent value and returns current
		if state.valid {
			return state.currentValue, nil
		}
		return phpv.ZNULL.ZVal(), nil
	}

	generatorAdvance(ctx, state, sendVal)

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
		return nil, ThrowError(ctx, TypeError, fmt.Sprintf("Generator::throw() expects parameter 1 to be Throwable, %s given", exc.GetType()))
	}
	throwErr := &phperr.PhpThrow{Obj: excObj}

	if !state.started {
		// Start the generator with a throw
		state.started = true
		state.status = GeneratorRunning

		state.resumeCh <- generatorMsg{err: throwErr}

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

	if state.status == GeneratorClosed {
		return nil, throwErr
	}

	return generatorThrowInner(ctx, o, state, throwErr)
}

func generatorGetReturn(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	state := getGeneratorState(o)
	if state == nil {
		return nil, ThrowError(ctx, Error, "Cannot get return value of a generator that hasn't returned")
	}

	if state.status != GeneratorClosed {
		return nil, ThrowError(ctx, Error, "Cannot get return value of a generator that hasn't returned")
	}

	if state.genErr != nil {
		return nil, ThrowError(ctx, Error, "Cannot get return value of a generator that threw an exception")
	}

	if state.returnVal == nil {
		return phpv.ZNULL.ZVal(), nil
	}
	return state.returnVal, nil
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
	generatorEnsureStarted(ctx, it.state)
	if !it.state.valid {
		return phpv.ZNULL.ZVal(), nil
	}
	return it.state.currentValue, nil
}

func (it *generatorIterator) Key(ctx phpv.Context) (*phpv.ZVal, error) {
	generatorEnsureStarted(ctx, it.state)
	if !it.state.valid {
		return phpv.ZNULL.ZVal(), nil
	}
	return it.state.currentKey, nil
}

func (it *generatorIterator) Next(ctx phpv.Context) (*phpv.ZVal, error) {
	generatorEnsureStarted(ctx, it.state)
	generatorAdvance(ctx, it.state, nil)
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
		generatorEnsureStarted(ctx, it.state)
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
	generatorEnsureStarted(ctx, it.state)
	return it.state.valid
}

func (it *generatorIterator) Iterate(ctx phpv.Context) iter.Seq2[*phpv.ZVal, *phpv.ZVal] {
	return func(yield func(*phpv.ZVal, *phpv.ZVal) bool) {
		generatorEnsureStarted(ctx, it.state)
		for it.state.valid {
			key := it.state.currentKey
			value := it.state.currentValue
			if !yield(key, value) {
				break
			}
			generatorAdvance(ctx, it.state, nil)
		}
	}
}
