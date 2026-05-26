package vm

// Op is a single-byte opcode identifier.
type Op uint8

// Opcodes for the MVP scope (literals, vars, arith, compare, jumps, ret).
// Function-call opcodes are added in a follow-up commit.
const (
	OpNop Op = iota

	// --- load / store ----------------------------------------------------
	OpLoadConst       // push CachedZ[A]  (scalar literal, A = const-pool index)
	OpLoadNull        // push ZNULL.ZVal()
	OpLoadTrue        // push ZTrue.ZVal()
	OpLoadFalse       // push ZFalse.ZVal()
	OpLoadLocal       // push ctx.OffsetGet(Locals[A])
	OpLoadLocalOrWarn // same but emit "Undefined variable" notice if missing
	OpStoreLocal      // pop -> ctx.OffsetSet(Locals[A], _)
	OpStoreLocalKeep  // top -> ctx.OffsetSet(Locals[A], _) (leaves value on stack)
	OpDup             // duplicate top of stack
	OpPop             // discard top of stack

	// --- arithmetic / bitwise (call OperatorMath / OperatorMathLogic) ----
	OpAdd
	OpSub
	OpMul
	OpDiv
	OpMod
	OpPow
	OpBitAnd
	OpBitOr
	OpBitXor
	OpShl
	OpShr
	OpNeg    // unary -
	OpBitNot // unary ~
	OpConcat // .

	// --- compare ---------------------------------------------------------
	OpCmpEq
	OpCmpNe
	OpCmpId  // ===
	OpCmpNid // !==
	OpCmpLt
	OpCmpLe
	OpCmpGt
	OpCmpGe
	OpCmpSpaceship // <=>

	// --- logical ---------------------------------------------------------
	OpNot  // unary !
	OpBool // cast top to bool

	// --- inc/dec on locals (call DoInc) ---------------------------------
	OpIncLocal     // pre-increment local A; pushes new value
	OpDecLocal     // pre-decrement local A; pushes new value
	OpPostIncLocal // post-increment local A; pushes OLD value
	OpPostDecLocal // post-decrement local A; pushes OLD value

	// --- compound assign (B = tokenizer.ItemType, A = local idx) --------
	// Stack: pops RHS; reads-modifies-writes Locals[A] using the op named by B.
	OpOpAssignLocal

	// --- control flow (signed C is a relative jump in instructions) -----
	OpJmp             // pc += C
	OpJmpIfFalse      // pop; if !v.AsBool() pc += C
	OpJmpIfTrue       // pop; if v.AsBool() pc += C
	OpJmpIfFalsePeek  // peek; if !v.AsBool() pc += C  (used by ||)
	OpJmpIfTruePeek   // peek; if v.AsBool() pc += C   (used by &&)
	OpJmpIfNotNullPeek // peek; if v.GetType() != ZtNull pc += C  (used by ??)

	// --- call / return ---------------------------------------------------
	// (CALL opcodes are reserved here so emitter constants don't shift
	// when they're implemented; the dispatcher returns ErrUnknownOp until
	// then so accidental emission fails loudly.)
	OpCallUser     // A = const-pool ZString name, B = argc
	OpCallDirect   // A = SubFns[A], B = argc
	OpCallIndirect // B = argc; pops argc args + callable from stack
	OpRet        // pop, exit frame with value
	OpRetNull    // exit frame with null

	// --- arrays ----------------------------------------------------------
	// OP_NEW_ARRAY pushes a new, empty *ZArray.ZVal() onto the stack.
	// Used as the start of building an array literal and for auto-viv
	// (the latter is currently in OP_ARRAY_*_LOCAL handlers).
	OpNewArray
	// OP_ARRAY_INIT_APPEND pops the value from the stack and appends it
	// to the *ZArray sitting one slot below — the array stays on top so
	// the next entry can be added without re-loading it. Used for the
	// un-keyed entries of a literal: `[1, 2, 3]`.
	OpArrayInitAppend
	// OP_ARRAY_INIT_KEYED pops value, then key. The *ZArray below is
	// mutated in place. Used for keyed entries in a literal:
	// `[k => v, ...]`.
	OpArrayInitKeyed
	// OP_ARRAY_GET pops offset, then container. Pushes container[offset].
	// Mirrors the AST runArrayAccess.Run on read (warns on undefined
	// keys, throws on string-as-array-of-letters edge cases). Containers
	// other than ZtArray fall back via the dispatcher's special-case
	// handling (string offsets, ArrayAccess interface — TODO).
	OpArrayGet
	// OP_ARRAY_SET_LOCAL pops the value, then the offset. The local at
	// index A is read; if it's null/uninitialized, a fresh *ZArray is
	// created and stored back. The offset is then assigned the value.
	// No result is left on the stack — used in statement context only.
	OpArraySetLocal
	// OP_ARRAY_APPEND_LOCAL pops the value. Local A is auto-vivified to
	// *ZArray if null; then the value is appended. Statement context
	// only — no result on the stack.
	OpArrayAppendLocal

	// --- exceptions ------------------------------------------------------
	// OP_THROW pops the value and throws it via phpobj.ThrowObject.
	OpThrow

	// --- closures --------------------------------------------------------
	// OP_MAKE_CLOSURE runs the *ZClosure template at SubClosures[A] via
	// its AST Run() method (which handles dup + capture + Spawn for
	// inline closures, and RegisterFunction for named declarations) and
	// pushes the result onto the stack. Statement-context emitters
	// follow with an OP_POP to discard the value.
	OpMakeClosure

	// OP_CLASS_CONST runs the embedded *runClassDynConst at SubASTs[A]
	// and pushes the resolved class constant value. We delegate to the
	// AST runner because constant resolution involves CompileDelayed
	// expressions, visibility checks, interface/parent walking, and
	// late-static-binding nuances that are intricate to lower
	// piecewise.
	OpClassConst
	// OP_TRY_FINALLY runs the embedded *runnableTry at SubASTs[A] and
	// pops/discards its (always-null) return value. Delegating the
	// whole try lets the AST orchestrate finally on every exit path
	// (normal completion, caught exception, uncaught exception,
	// return/break/continue inside the try body).
	OpTryFinally
	// OP_REFRESH_SLOTS re-reads every local from the FuncContext
	// hashtable into the slot cache. Emit it after any AST-delegated
	// opcode whose Run() may have written locals via ctx.OffsetSet
	// (property writes, etc.) — otherwise subsequent slot reads see
	// the stale pre-delegation values.
	OpRefreshSlots

	// --- objects ---------------------------------------------------------
	// OP_NEW_OBJECT pops B args, looks up class by Consts[A] (a ZString),
	// instantiates via phpobj.NewZObject (which handles abstract/interface/
	// enum errors and __construct dispatch), pushes the new object.
	OpNewObject
	// OP_OBJECT_GET pops the receiver and pushes receiver->name where
	// name is Consts[A] (a ZString). Dispatches at runtime: ZtObject
	// uses ObjectGet (which handles __get and visibility), null/bool
	// receivers warn + return null, scalars produce the standard error.
	OpObjectGet
	// OP_OBJECT_SET pops the value, then the receiver. Sets
	// receiver->name = value. Name is Consts[A] (a ZString).
	// If B != 0, the value is pushed back on the stack after the set
	// (for expression-context `$x = ($obj->prop = v)` semantics).
	OpObjectSet
	// OP_OBJECT_CALL pops B args and the receiver. Pushes the result
	// of receiver->name(args). Name is Consts[A] (a ZString).
	OpObjectCall

	// --- foreach ---------------------------------------------------------
	// OP_FOREACH_INIT pops the src container, builds an iterator, and
	// pushes it onto the frame's iter stack. If src isn't iterable it
	// emits the PHP "argument must be of type array|object" warning
	// and jumps by C (skipping the entire loop body and unwind).
	// Otherwise it falls through.
	OpForeachInit
	// OP_FOREACH_STEP checks the current iterator. If exhausted, it
	// pops the iterator and jumps by C. Otherwise it stores the
	// current value into local A; if B != 0xFFFF it also stores the
	// current key into local B. Does NOT advance — that's
	// OP_FOREACH_ADVANCE.
	OpForeachStep
	// OP_FOREACH_ADVANCE calls Next() on the top iterator.
	OpForeachAdvance
	// OP_FOREACH_UNWIND pops the top iterator (used at the loop's
	// natural exit and as a `break` target).
	OpForeachUnwind

	// --- diagnostics -----------------------------------------------------
	OpTick // call ctx.Tick(ctx, LocAt(pc-1)) and DrainTempObjects

	// --- type / class introspection -------------------------------------
	// OP_INSTANCEOF pops the value and a class name (ZString) and pushes
	// the boolean result of `$v instanceof $cls`. Both static and dynamic
	// class names share the same opcode; the emitter handles the cls
	// resolution before this op (either OP_LOAD_CONST for a literal name
	// or an expression that produces the class name).
	OpInstanceOf
	// OP_CLONE pops the value and pushes a cloned copy via
	// compiler.EvalClone (basic form only — `clone $x`). The PHP 8.5+
	// extended forms (`clone($x, $with)`, `clone(...$arr)`, named args)
	// remain AST-delegated.
	OpClone
	// OP_CLASS_NAMEOF pops the class-source value and pushes the
	// resolved class name (a ZString) via compiler.EvalClassNameOf.
	// Implements both `Foo::class` and `$obj::class`. A non-zero A flag
	// signals that the source expression was a compile-time literal,
	// which affects the error message when the value isn't usable as
	// a class name.
	OpClassNameOf
	// OP_INLINE_HTML writes the string at Consts[A] directly to the
	// output stream. Replaces the AST runInlineHtml.Run path.
	OpInlineHtml
	// OP_SET_STRICT_TYPES calls ctx.Global().SetStrictTypes(true).
	// Replaces the AST runnableDeclareStrictTypes.Run path.
	OpSetStrictTypes
	// OP_FIRSTCLASS_CALLABLE pops a value and pushes a Closure
	// produced by compiler.ClosureFromCallable. Used by the PHP 8.1
	// `func(...)` first-class callable syntax for free functions.
	OpFirstClassCallable
	// OP_FIRSTCLASS_CLONE pushes a Closure that wraps the clone
	// built-in. Used by the PHP 8.5+ `clone(...)` first-class
	// callable syntax. Takes no operand.
	OpFirstClassClone
	// OP_METHOD_FIRSTCLASS pops the receiver and pushes a Closure for
	// the method first-class callable form. A's bits: bit0 = static
	// (`Cls::method(...)`), bit1 = nullsafe (`$x?->method(...)`).
	// The method name is at Consts[B] (a ZString).
	OpMethodFirstClass
	// OP_DYN_METHOD_FIRSTCLASS pops the method-name value and the
	// receiver, then pushes the Closure. Used by the
	// `$obj->{expr}(...)` dynamic-name method first-class form.
	OpDynMethodFirstClass
	// OP_OBJECT_DYN_GET pops the name value and the receiver, then
	// pushes the result of `$obj->{$name}`. Plain (non-nullsafe) form
	// only; nullsafe and nullChain variants AST-delegate.
	OpObjectDynGet
	// OP_GLOBAL_BIND pops the name value and binds the local of that
	// name to the global slot via compiler.EvalGlobalBinding.
	OpGlobalBind
	// OP_CLASS_DYN_CONST pops the const-name value and the class-source
	// value (class-name string, object instance, or `self`/`parent`/
	// `static` magic string) and pushes the resolved class constant.
	// Delegates to compiler.EvalClassDynConst. Implements `Cls::CONST`,
	// `$obj::CONST`, `Cls::{$name}`, and `$obj::class`-as-const-name.
	OpClassDynConst
	// OP_CLASS_STATIC_GET pops the class-source value and pushes the
	// value of the static property at Consts[A] (a ZString). Read-only
	// path; writes are handled by emitAssignViaAST.
	OpClassStaticGet
	// OP_CLASS_STATIC_OBJREF pops the class-source value and pushes
	// the resolved class constant / enum case at Consts[A] (a ZString).
	// Handles trait blocking, visibility, attribute deprecation,
	// CompileDelayed + [constant expression] frame decoration, enum
	// errors, and typed-const coercion via compiler.EvalClassStaticObjRef.
	OpClassStaticObjRef
	// OP_CLASS_STATIC_DYN_GET pops the name value and the class-source
	// value, then pushes the value of the dynamically-named static
	// property. No visibility check (matching original AST semantics).
	OpClassStaticDynGet
	// OP_CALL_TICK_FUNCTIONS calls ctx.Global().CallTickFunctions(ctx).
	// Used by `declare(ticks=N)` bodies, inserted by the emitter after
	// every N statements within the body.
	OpCallTickFunctions
	// OP_DEFINE_CONST pops the evaluated value of a top-level
	// `const NAME = expr;` definition and registers it in the global
	// constant table. The name / attributes / source location come from
	// SubASTs[A] (the *runTopLevelConst node).
	OpDefineConst
	// OP_ARRAY_SPREAD_APPEND pops the spread source value and appends
	// its contents into the in-progress array on top of stack via
	// compiler.SpreadIntoArray. Used by `[…, ...$expr, …]` literals.
	OpArraySpreadAppend
	// OP_ISSET_LOCAL pushes a bool: the local at slot A is set (non-nil
	// and non-null). The simple `isset($x)` form; multi-arg isset is
	// emitted as a short-circuit chain of these opcodes.
	OpIssetLocal
	// OP_EMPTY_LOCAL pushes a bool: the local at slot A is empty per
	// PHP's `empty(…)` rules (unset, null, false, 0, "", "0", []).
	OpEmptyLocal
	// OP_UNSET_LOCAL clears the local at slot A: fires the destructor
	// for any object value, nils out the slot, and calls OffsetUnset
	// so external observers see the removal. Used by the simple form
	// of `unset($x)`.
	OpUnsetLocal
	// OP_THROW_UNHANDLED_MATCH pops the match condition value and
	// throws an UnhandledMatchError formatted with the value. Used at
	// the no-default fall-through of a native `match (…)` lowering.
	OpThrowUnhandledMatch
	// OP_REGISTER_ENUM runs the enum-specific registration + Compile
	// + validation flow on SubASTs[A] (the *runEnumRegister node) via
	// compiler.RegisterEnum. Used by the native `enum Foo { … }`
	// statement emit.
	OpRegisterEnum
	// OP_NODISCARD_ENTER clears LastCallable, sets inNoDiscardContext
	// to true, and stores the previous value as a bool in local slot A
	// so OP_NODISCARD_EXIT can restore it. Used by the native lowering
	// of `#[NoDiscard]`-wrapped statements.
	OpNoDiscardEnter
	// OP_NODISCARD_EXIT reads the previous flag from local slot A,
	// inspects LastCallable for a NoDiscard attribute, emits the
	// warning if appropriate, and restores inNoDiscardContext.
	OpNoDiscardExit
	// OP_LOAD_CONSTANT_BY_NAME resolves a user/namespaced/built-in
	// constant via compiler.LookupConstant and pushes its value.
	// Consts[A] holds the literal name (ZString); bit 0 of B is the
	// noFallback flag (set when the name starts with `\` or was
	// matched via use-const aliases).
	OpLoadConstantByName
	// OP_VAR_VAR_READ pops the name expression's value, coerces it to
	// string, and pushes the value of the variable with that name.
	// Bit 0 of A is the warn flag — set in read contexts (subexpression
	// position) to emit the "Undefined variable" notice when the name
	// resolves to nothing.
	OpVarVarRead
	// OP_NEW_ANON_CLASS instantiates the `new class { … }` whose AST
	// node lives at SubASTs[A]. Pops B constructor arguments off the
	// stack (in order), ensures the class is registered + compiled
	// (idempotent), and pushes the resulting object. Used only for
	// constructor-arg shapes that don't need by-ref auto-vivification.
	OpNewAnonClass
	// OP_ARRAY_GET_SAFE pops key, then container; pushes
	// compiler.IssetChainElement(ctx, container, key) — the value if
	// accessible per isset's permissive read semantics, or null if
	// missing / non-accessible / null-resulting. Used for the
	// intermediate steps of nested `isset($a[k1][k2][k3])` so a missing
	// intermediate doesn't throw TypeError on the next-level read.
	OpArrayGetSafe
	// OP_ISSET_DIM pops key, then container; pushes a bool from
	// compiler.EvalIssetDim — "exists & not null" for arrays/strings/
	// ArrayAccess containers with the same edge cases the AST runner
	// handles (null-key deprecation, array/object-key TypeErrors,
	// string-offset coercion, FindIssetDimHandler dispatch).
	OpIssetDim
	// OP_EMPTY_DIM pops key, then container; pushes a bool from
	// compiler.EvalEmptyDim — mirrors `empty($c[$k])` for the same
	// container shapes.
	OpEmptyDim
	// OP_UNSET_DIM pops key, then container; removes container[key]
	// in place via compiler.UnsetArrayDim. Used by the native form of
	// `unset($a[$k])` where the container is a simple variable; nested
	// shapes (`$a[$k1][$k2]`) and non-local containers still AST-
	// delegate via the AST WriteValue.
	OpUnsetDim
	// OP_COERCE_RETURN pops the return value, applies non-strict
	// early type coercion against the function's return type hint
	// (stored in SubASTs[A] as the *runReturn node), and pushes the
	// coerced value. Used in the native typed-return lowering ahead
	// of OP_RET so a `function foo(): int { return 1.5; }` emits the
	// "Implicit conversion from float to int" deprecation at the
	// return statement (before any finally runs).
	OpCoerceReturn
	// OP_STATIC_PROP_SET pops the value, pops the class-source value,
	// and writes the static property named Consts[A] on the resolved
	// class via compiler.AssignClassStaticProp. The helper handles
	// LSB-aware class resolution, asymmetric visibility, typed-prop
	// enforcement (strict + weak coercion), and IncRef/DecRef.
	// If B != 0, the value is pushed back on the stack after the set
	// (for expression-context `$x = (Foo::$bar = v)` semantics).
	OpStaticPropSet
	// OP_CREATE_REF pushes a reference to the target at SubASTs[A]
	// (a *runRef). The opcode dispatches through compiler.EvalCreateRef
	// which handles per-shape semantics (variable / array-access /
	// object-prop / static-prop / non-variable expression).
	OpCreateRef
	// OP_CLONE_EXT pushes the result of an extended PHP 8.5+ clone
	// expression — `clone($x, $with)`, `clone(...$arr)`, or
	// clone with named args. The *runnableClone node lives at
	// SubASTs[A]; compiler.EvalCloneExt does the work. The basic
	// `clone $x` form uses OP_CLONE; this is the rare-extended-form
	// dedicated dispatch.
	OpCloneExt
	// OP_STATIC_METHOD_CALL dispatches a `Foo::method(args)` static
	// method call (also `self::`, `parent::`, `static::`). The
	// *runObjectFunc node at SubASTs[A] carries the class-source
	// expression, method name, and arg list. compiler.EvalStaticMethodCall
	// does class resolution, LSB binding, and dispatch.
	OpStaticMethodCall
	// OP_STATIC_VAR_BIND runs `static $x = init; …` for the
	// *runStaticVar at SubASTs[A]: evaluates each entry's initializer
	// the first time per scope key (closure-instance, class, or
	// global), stashes the cell on the AST node, and installs the
	// cell as the current scope's local for $x.
	OpStaticVarBind
	// OP_DESTRUCTURE_ASSIGN pops the RHS value and writes it into the
	// `list(…)` / `[…]` LHS at SubASTs[A] via compiler.AssignDestructure,
	// which handles nested destructure entries, keyed entries, and
	// ArrayAccess-object sources. The RHS value is also pushed back
	// (so the assignment expression can produce a value in non-stmt
	// context) when B's bit 0 is set.
	OpDestructureAssign

	// OP_FINALLY_END marks the end of a finally body in the native
	// try-with-finally lowering. At runtime it inspects the frame's
	// pending-control register and either:
	//   - pending=none: falls through to the next instruction (normal
	//     completion of try + finally, or catch + finally).
	//   - pending=return: re-attempts the return — chains into an outer
	//     finally (Start <= pc < FinallyPC of an outer TryHandler) if
	//     one covers this PC, else exits the frame returning pending.val.
	//   - pending=throw: re-raises the held *phperr.PhpThrow so the
	//     outer dispatch can route it to a catch or an outer finally.
	// The opcode encodes no operands; the pending register is set by
	// OP_RET / OP_RET_NULL / dispatchTryHandler when they detect they're
	// crossing an enclosing finally.
	OpFinallyEnd

	// OP_FOREACH_STEP_PUSH is the stack-pushing variant of OP_FOREACH_STEP,
	// used for foreach value/key targets that aren't a bare local (e.g.
	// `foreach ($arr as [$a, $b])` or `as $obj->prop => $val`). If the
	// iterator is exhausted it jumps by C as OP_FOREACH_STEP does;
	// otherwise it pushes the current value onto the stack (always
	// Dup()'d to match the AST runner's snapshot semantics). When A != 0
	// the current key is pushed FIRST (below the value), so the emitter
	// can pop+assign value then pop+assign key — matching the AST's
	// "key write before value write" order.
	OpForeachStepPush

	// OP_ASSIGN_WRITABLE pops a value off the stack and writes it to
	// the AST Writable node at SubASTs[A] via WriteValue. Used for
	// foreach targets whose shape isn't a bare local (destructure,
	// object prop, array element, …) so we can drive the loop natively
	// while delegating only the per-iteration write.
	OpAssignWritable

	// OP_CALL_USER_BY_EXPRS is the by-ref / named / spread variant of
	// OP_CALL_USER. A = const-pool ZString function name (with the same
	// CallableCache hookup OP_CALL_USER uses); B = SubArgs index of the
	// arg-expression list. The handler resolves the callable then calls
	// ctx.Call(ctx, callable, exprs, nil), which evaluates each arg
	// expression with by-ref binding, named-arg reordering, and spread
	// expansion. Pushes the result on top of the stack. No args are
	// pushed beforehand — the expressions live in fn.SubArgs.
	OpCallUserByExprs

	// OP_CALL_INDIRECT_BY_EXPRS is the by-ref / named / spread variant
	// of OP_CALL_INDIRECT. A = SubArgs index of the arg-expression list.
	// The callable is at the top of the stack (already emitted as a
	// native expression); the handler pops it, resolves it via
	// ResolveCallable, then calls ctx.Call. Pushes the result.
	OpCallIndirectByExprs

	// OP_OBJECT_CALL_BY_EXPRS is the by-ref / named / spread variant of
	// instance method calls. A = const-pool ZString method name; B =
	// SubArgs index of the arg-expression list. The receiver is at the
	// top of the stack; the handler pops it, resolves the method, then
	// calls ctx.Call with the receiver as $this. Pushes the result.
	OpObjectCallByExprs

	// OP_OBJECT_DYN_CALL dispatches a `$obj->{$expr}(args)` (or
	// `Foo::{$expr}(args)`) dynamic-method-name call. SubASTs[A] holds
	// the *runObjectDynFunc node carrying the receiver / name-expr /
	// arg-expr list plus the static / nullsafe / nullChain flags;
	// compiler.EvalObjectDynFunc runs the node's body verbatim
	// (receiver evaluation, name evaluation, dispatch via ctx.Call
	// with by-ref/named/spread binding). Pushes the result. Replaces
	// the generic OpClassConst delegation for this AST node so the
	// emit-side path is dedicated.
	OpObjectDynCall

	// OP_OBJECT_GET_SAFE is the permissive variant of OP_OBJECT_GET used
	// as the LHS of `$obj->prop ?? default`. Same operand encoding as
	// OP_OBJECT_GET (A = const-pool name index, B = nullsafe flag), but
	// suppresses the "Undefined property" warning on ZtObject receivers
	// — missing properties just yield null. Non-object receivers still
	// emit "Attempt to read property on int/bool/..." warnings since
	// PHP's `??` only silences property-existence, not receiver-type.
	OpObjectGetSafe

	// OP_ISSET_OBJ_PROP / OP_EMPTY_OBJ_PROP — native `isset($obj->prop)` /
	// `empty($obj->prop)` for static-name, non-nullsafe property access.
	// Encoding: A = const-pool name index. Pops receiver, pushes bool.
	// Dispatches to compiler.EvalIssetObjProp / EvalEmptyObjProp which
	// mirror checkExistence / checkEmpty's runObjectVar branches.
	OpIssetObjProp
	OpEmptyObjProp

	// `unset($obj->prop)` for static-name, non-nullsafe property
	// access. Encoding: A = const-pool name index. Pops receiver,
	// pushes nothing. Dispatches to compiler.EvalUnsetObjProp which
	// mirrors the value==nil branch of runObjectVar.WriteValue —
	// non-object receivers are silently ignored, object receivers
	// dispatch to ObjectSet(name, nil).
	OpUnsetObjProp

	// `$obj->prop OP= rhs` compound assign for static-name, non-
	// nullsafe property access. Encoding:
	//   A = const-pool name index (ZString prop name)
	//   B = tokenizer ItemType (compound op token, e.g.
	//       T_PLUS_EQUAL, T_CONCAT_EQUAL)
	//   C bit 0 = "keep value on stack" (expr-context vs stmt-ctx)
	// Stack: pops rhs, pops receiver. Reads cur via objectGet,
	// applies compoundOp(op)(cur, rhs), writes back via objectSet.
	// Pushes the post-op value back when keep-flag is set.
	OpObjectCompoundAssign

	// `Foo::$bar OP= rhs` static-property compound assign. Encoding
	// mirrors OP_OBJECT_COMPOUND_ASSIGN:
	//   A = const-pool name index (ZString varName, w/o leading $)
	//   B = tokenizer ItemType
	//   C bit 0 = keep-value-on-stack flag
	// Stack: pops rhs, pops class-source. Reads via
	// EvalClassStaticVarRead, applies compoundOp, writes back via
	// AssignClassStaticProp. The two helpers carry the LSB-aware
	// class resolution, visibility checks, and typed-prop coercion.
	OpStaticPropCompoundAssign

	// `$obj->prop++` / `++$obj->prop` / `--` variants. Encoding:
	//   A = const-pool name index (ZString prop name)
	//   B bit 0 = inc (1) vs dec (0)
	//   B bit 1 = post (1) vs pre (0)
	//   C bit 0 = keep-value-on-stack (pre/post-mut value)
	// Stack: pops receiver. Reads cur via objectGet, applies
	// compiler.DoInc (which handles the string-increment "a"->"b",
	// NULL->1 cases), writes back via objectSet. Pushes the value
	// requested by C+post.
	OpIncDecObjProp

	// `Foo::$bar++` / `++Foo::$bar` / `--` variants. Same encoding
	// as OP_INC_DEC_OBJ_PROP but the stack pops a class-source. The
	// read/write go through EvalClassStaticVarRead and
	// AssignClassStaticProp.
	OpIncDecStaticProp

	// `isset(Foo::$bar)` / `empty(Foo::$bar)` for static-name access.
	// Encoding: A = const-pool name index. Pops class-source, pushes
	// bool. Dispatches to compiler.EvalIssetStaticProp /
	// EvalEmptyStaticProp which mirror checkExistence /
	// checkEmpty's *runClassStaticVarRef branches.
	OpIssetStaticProp
	OpEmptyStaticProp

	// OP_OBJECT_DYN_SET handles both `$obj->$x = v` (runObjectVar with
	// $-prefixed name) and `$obj->{$x} = v` (runObjectDynVar). Stack
	// pops value, name, receiver (in that order). Encoding:
	//   A bit 0 = keep-value-on-stack (expr context)
	// Handler checks receiver is a ZObjectAccess (throws
	// "Attempt to assign property \"NAME\" on TYPE" otherwise) and
	// dispatches to objI.ObjectSet which handles typed properties,
	// hooks, asymmetric visibility, etc.
	OpObjectDynSet

	// OP_STATIC_PROP_DYN_SET handles `Cls::${$x} = v` and `Cls::$$x = v`.
	// Stack pops value, name, class-source (in that order). Encoding:
	//   A bit 0 = keep-value-on-stack (expr context)
	// Dispatches to compiler.AssignClassStaticDynProp which resolves
	// the class and writes the property (no visibility / asymmetric /
	// type-hint check — matches AST semantics).
	OpStaticPropDynSet

	// OP_VAR_VAR_SET handles `$$name = v` / `${$expr} = v`.
	//
	// Encoding: A bit 0 = keep value on stack (expression context).
	//
	// Stack effect (stmt context):  pops name, value
	// Stack effect (expr context):  pops name, value, pushes value
	//
	// Dispatches to compiler.AssignVariableVariable which coerces the
	// name to string and writes via ctx.OffsetSet. Mirrors the AST
	// runVariableRef.WriteValue body.
	OpVarVarSet

	// OP_STATIC_PROP_DYN_COMPOUND_ASSIGN handles
	// `Cls::${$x} OP= rhs` dynamic-name static-prop compound assigns.
	//
	// Encoding:
	//   B = compound op kind (token, e.g. T_PLUS_EQUAL)
	//   C bit 0 = keep value on stack (expression context)
	//
	// Stack effect (stmt): pops class-source, name, rhs
	// Stack effect (expr): pops class-source, name, rhs; pushes result
	//
	// Reads via compiler.EvalClassStaticDynVarRead, applies compoundOp,
	// writes back via compiler.AssignClassStaticDynProp. Matches the AST
	// dyn-name path (no asymmetric-visibility / typed-prop check —
	// that's a pre-existing gap of the dyn-name path).
	OpStaticPropDynCompoundAssign

	// OP_INC_DEC_STATIC_PROP_DYN handles `Cls::${$x}++` / `++Cls::${$x}`
	// (and the dec variants) for dynamic-name static properties.
	//
	// Encoding:
	//   B bit 0 = increment (1) vs decrement (0)
	//   B bit 1 = postfix (1) vs prefix (0)
	//   C bit 0 = keep value on stack (expression context)
	//
	// Stack effect (stmt): pops class-source, name
	// Stack effect (expr): pops class-source, name; pushes pre/post value
	//
	// Reads via EvalClassStaticDynVarRead, applies DoInc, writes via
	// AssignClassStaticDynProp. Mirrors OP_INC_DEC_STATIC_PROP for the
	// dyn-name form.
	OpIncDecStaticPropDyn

	// OP_OBJECT_DYN_COMPOUND_ASSIGN handles `$obj->$x OP= rhs` and
	// `$obj->{$x} OP= rhs` dynamic-name property compound assigns.
	//
	// Encoding:
	//   B = compound op kind (token, e.g. T_PLUS_EQUAL)
	//   C bit 0 = keep value on stack (expression context)
	//
	// Stack effect (stmt): pops receiver, name, rhs
	// Stack effect (expr): pops receiver, name, rhs; pushes result
	//
	// Reads via objectGet, applies compoundOp, writes via objectSet
	// (both internal VM helpers that dispatch through ZObject's typed/
	// hook/asymmetric machinery).
	OpObjectDynCompoundAssign

	// OP_INC_DEC_OBJ_DYN_PROP handles `$obj->$x++` / `++$obj->$x` and
	// `$obj->{$x}++` / `++$obj->{$x}` plus the dec variants for
	// dynamic-name properties.
	//
	// Encoding:
	//   B bit 0 = increment (1) vs decrement (0)
	//   B bit 1 = postfix (1) vs prefix (0)
	//   C bit 0 = keep value on stack (expression context)
	//
	// Stack effect (stmt): pops receiver, name
	// Stack effect (expr): pops receiver, name; pushes pre/post value
	//
	// Reads via objectGet, applies DoInc, writes via objectSet.
	OpIncDecObjDynProp

	// OP_STORE_LOCAL_REF stores a reference-typed ZVal into a local
	// slot without dereferencing or COW-Dup'ing. Used for `$b = &$a`
	// reference assignments where the RHS is `&$expr` (a runRef
	// expression — produces a ref-typed ZVal via OP_CREATE_REF).
	//
	// Encoding:
	//   A = local slot index
	//   B bit 0 = keep value on stack (expression context, e.g.
	//             `$x = ($b = &$a)` — the result of `$b = &$a` is
	//             the ref ZVal which then gets dereferenced by the
	//             outer `=`'s value semantics in OpStoreLocal)
	//
	// Differs from OP_STORE_LOCAL: that handler Dup's ZtArray values
	// even when the source is a ref (frame.go:160-166), which detaches
	// the ref. This handler writes the ref ZVal directly and calls
	// RefInner() to track the alias count — matches the AST's
	// `res.IsRef() && r.b is *runRef` branch in run-operator.go.
	OpStoreLocalRef

	// OP_ARRAY_COMPOUND_ASSIGN_LOCAL handles `$local[offset] OP= rhs`
	// where the LHS container is a simple local variable. Reads the
	// current element via arrayGet, snapshots it (defending against
	// handlers that mutate the slot during `.=` array→string coercion,
	// bug81705-family), applies the resolved compoundOp, then writes
	// back via arraySetLocal.
	//
	// Stack on entry (top-down): rhs, offset.
	// Pops rhs, offset; in expr context (C bit 0) pushes the result.
	//
	// Encoding:
	//   A = local slot index
	//   B = compound op token (T_PLUS_EQUAL, T_CONCAT_EQUAL, …)
	//   C bit 0 = keep result on stack
	OpArrayCompoundAssignLocal
	// OP_ARRAY_INC_DEC_LOCAL handles `$local[offset]++`, `++$local[offset]`
	// and dec variants on a simple-local array container. Reads the
	// current element via arrayGet, Dup's it (DoInc mutates in place),
	// applies DoInc, writes back via arraySetLocal. In expr context,
	// pushes the original value for postfix or the new value for prefix.
	//
	// Stack on entry: offset (one operand).
	// Pops offset; in expr context (C bit 0) pushes the result.
	//
	// Encoding:
	//   A = local slot index
	//   B bit 0 = inc (1) / dec (0)
	//   B bit 1 = post (1) / pre (0)
	//   C bit 0 = keep result on stack
	OpArrayIncDecLocal

	// OP_ARRAY_PRE_CHECK_LOCAL runs BEFORE the offset is evaluated in
	// a `$local[offset] OP= rhs` or `$local[offset]++` operation. It
	// mirrors the AST's compound-write-context handling of the container
	// in runArrayAccess.Run so that warnings about an undefined container
	// fire BEFORE warnings emitted by offset evaluation. Specifically:
	//   - Slot is nil (undefined): warn "Undefined variable $X" and
	//     vivify the slot to an empty array.
	//   - Slot holds ZtNull: vivify to empty array (no warning — explicit
	//     null assignment, like the AST's auto-vivify path).
	//   - Slot holds ZtString: throw "Cannot use assign-op operators with
	//     string offsets" before the offset is touched (bug53432).
	// Other container types (array, object, bool true, scalar) are left
	// untouched and any error/auto-vivify is handled downstream by
	// arrayGet/arraySetLocal.
	//
	// Stack: no changes (no operands popped or pushed).
	//
	// Encoding:
	//   A = local slot index
	OpArrayPreCheckLocal

	// Sentinel — keep last.
	opLast
)

// String returns the symbolic name of the opcode (for disassembly).
func (o Op) String() string {
	if int(o) < len(opNames) {
		if s := opNames[o]; s != "" {
			return s
		}
	}
	return "OP?"
}

var opNames = [...]string{
	OpNop:             "NOP",
	OpLoadConst:       "LOAD_CONST",
	OpLoadNull:        "LOAD_NULL",
	OpLoadTrue:        "LOAD_TRUE",
	OpLoadFalse:       "LOAD_FALSE",
	OpLoadLocal:       "LOAD_LOCAL",
	OpLoadLocalOrWarn: "LOAD_LOCAL_OR_WARN",
	OpStoreLocal:      "STORE_LOCAL",
	OpStoreLocalKeep:  "STORE_LOCAL_KEEP",
	OpDup:             "DUP",
	OpPop:             "POP",
	OpAdd:             "ADD",
	OpSub:             "SUB",
	OpMul:             "MUL",
	OpDiv:             "DIV",
	OpMod:             "MOD",
	OpPow:             "POW",
	OpBitAnd:          "BITAND",
	OpBitOr:           "BITOR",
	OpBitXor:          "BITXOR",
	OpShl:             "SHL",
	OpShr:             "SHR",
	OpNeg:             "NEG",
	OpBitNot:          "BITNOT",
	OpConcat:          "CONCAT",
	OpCmpEq:           "CMP_EQ",
	OpCmpNe:           "CMP_NE",
	OpCmpId:           "CMP_ID",
	OpCmpNid:          "CMP_NID",
	OpCmpLt:           "CMP_LT",
	OpCmpLe:           "CMP_LE",
	OpCmpGt:           "CMP_GT",
	OpCmpGe:           "CMP_GE",
	OpCmpSpaceship:    "CMP_SPACESHIP",
	OpNot:             "NOT",
	OpBool:            "BOOL",
	OpIncLocal:        "INC_LOCAL",
	OpDecLocal:        "DEC_LOCAL",
	OpPostIncLocal:    "POSTINC_LOCAL",
	OpPostDecLocal:    "POSTDEC_LOCAL",
	OpOpAssignLocal:   "OP_ASSIGN_LOCAL",
	OpJmp:             "JMP",
	OpJmpIfFalse:      "JMP_IF_FALSE",
	OpJmpIfTrue:       "JMP_IF_TRUE",
	OpJmpIfFalsePeek:   "JMP_IF_FALSE_PEEK",
	OpJmpIfTruePeek:    "JMP_IF_TRUE_PEEK",
	OpJmpIfNotNullPeek: "JMP_IF_NOT_NULL_PEEK",
	OpCallUser:        "CALL_USER",
	OpCallDirect:      "CALL_DIRECT",
	OpCallIndirect:    "CALL_INDIRECT",
	OpRet:              "RET",
	OpRetNull:          "RET_NULL",
	OpNewArray:         "NEW_ARRAY",
	OpArrayInitAppend:  "ARRAY_INIT_APPEND",
	OpArrayInitKeyed:   "ARRAY_INIT_KEYED",
	OpArrayGet:         "ARRAY_GET",
	OpArraySetLocal:    "ARRAY_SET_LOCAL",
	OpArrayAppendLocal: "ARRAY_APPEND_LOCAL",
	OpThrow:            "THROW",
	OpMakeClosure:      "MAKE_CLOSURE",
	OpClassConst:       "CLASS_CONST",
	OpTryFinally:       "TRY_FINALLY",
	OpRefreshSlots:     "REFRESH_SLOTS",
	OpNewObject:        "NEW_OBJECT",
	OpObjectGet:        "OBJECT_GET",
	OpObjectSet:        "OBJECT_SET",
	OpObjectCall:       "OBJECT_CALL",
	OpForeachInit:      "FOREACH_INIT",
	OpForeachStep:      "FOREACH_STEP",
	OpForeachAdvance:   "FOREACH_ADVANCE",
	OpForeachUnwind:    "FOREACH_UNWIND",
	OpTick:             "TICK",
	OpInstanceOf:       "INSTANCEOF",
	OpClone:            "CLONE",
	OpClassNameOf:      "CLASS_NAMEOF",
	OpInlineHtml:       "INLINE_HTML",
	OpSetStrictTypes:      "SET_STRICT_TYPES",
	OpFirstClassCallable:  "FIRSTCLASS_CALLABLE",
	OpFirstClassClone:     "FIRSTCLASS_CLONE",
	OpMethodFirstClass:    "METHOD_FIRSTCLASS",
	OpDynMethodFirstClass: "DYN_METHOD_FIRSTCLASS",
	OpObjectDynGet:        "OBJECT_DYN_GET",
	OpGlobalBind:          "GLOBAL_BIND",
	OpClassDynConst:       "CLASS_DYN_CONST",
	OpClassStaticGet:      "CLASS_STATIC_GET",
	OpClassStaticObjRef:   "CLASS_STATIC_OBJREF",
	OpClassStaticDynGet:   "CLASS_STATIC_DYN_GET",
	OpCallTickFunctions:   "CALL_TICK_FUNCTIONS",
	OpDefineConst:         "DEFINE_CONST",
	OpArraySpreadAppend:   "ARRAY_SPREAD_APPEND",
	OpIssetLocal:          "ISSET_LOCAL",
	OpEmptyLocal:          "EMPTY_LOCAL",
	OpUnsetLocal:          "UNSET_LOCAL",
	OpThrowUnhandledMatch: "THROW_UNHANDLED_MATCH",
	OpRegisterEnum:        "REGISTER_ENUM",
	OpNoDiscardEnter:      "NODISCARD_ENTER",
	OpNoDiscardExit:       "NODISCARD_EXIT",
	OpLoadConstantByName:  "LOAD_CONSTANT_BY_NAME",
	OpVarVarRead:          "VAR_VAR_READ",
	OpNewAnonClass:        "NEW_ANON_CLASS",
	OpArrayGetSafe:        "ARRAY_GET_SAFE",
	OpIssetDim:            "ISSET_DIM",
	OpEmptyDim:            "EMPTY_DIM",
	OpUnsetDim:            "UNSET_DIM",
	OpCoerceReturn:        "COERCE_RETURN",
	OpStaticPropSet:       "STATIC_PROP_SET",
	OpCreateRef:           "CREATE_REF",
	OpCloneExt:            "CLONE_EXT",
	OpStaticMethodCall:    "STATIC_METHOD_CALL",
	OpStaticVarBind:       "STATIC_VAR_BIND",
	OpDestructureAssign:   "DESTRUCTURE_ASSIGN",
	OpFinallyEnd:          "FINALLY_END",
	OpForeachStepPush:     "FOREACH_STEP_PUSH",
	OpAssignWritable:      "ASSIGN_WRITABLE",
	OpCallUserByExprs:     "CALL_USER_BY_EXPRS",
	OpCallIndirectByExprs: "CALL_INDIRECT_BY_EXPRS",
	OpObjectCallByExprs:   "OBJECT_CALL_BY_EXPRS",
	OpObjectDynCall:       "OBJECT_DYN_CALL",
	OpObjectGetSafe:       "OBJECT_GET_SAFE",
	OpIssetObjProp:        "ISSET_OBJ_PROP",
	OpEmptyObjProp:        "EMPTY_OBJ_PROP",
	OpUnsetObjProp:        "UNSET_OBJ_PROP",
	OpObjectCompoundAssign: "OBJECT_COMPOUND_ASSIGN",
	OpStaticPropCompoundAssign: "STATIC_PROP_COMPOUND_ASSIGN",
	OpIncDecObjProp:            "INC_DEC_OBJ_PROP",
	OpIncDecStaticProp:         "INC_DEC_STATIC_PROP",
	OpIssetStaticProp:          "ISSET_STATIC_PROP",
	OpEmptyStaticProp:          "EMPTY_STATIC_PROP",
	OpObjectDynSet:             "OBJECT_DYN_SET",
	OpStaticPropDynSet:         "STATIC_PROP_DYN_SET",
	OpVarVarSet:                "VAR_VAR_SET",
	OpStaticPropDynCompoundAssign: "STATIC_PROP_DYN_COMPOUND_ASSIGN",
	OpIncDecStaticPropDyn:         "INC_DEC_STATIC_PROP_DYN",
	OpObjectDynCompoundAssign:     "OBJECT_DYN_COMPOUND_ASSIGN",
	OpIncDecObjDynProp:            "INC_DEC_OBJ_DYN_PROP",
	OpStoreLocalRef:               "STORE_LOCAL_REF",
	OpArrayCompoundAssignLocal:    "ARRAY_COMPOUND_ASSIGN_LOCAL",
	OpArrayIncDecLocal:            "ARRAY_INC_DEC_LOCAL",
	OpArrayPreCheckLocal:          "ARRAY_PRE_CHECK_LOCAL",
}
