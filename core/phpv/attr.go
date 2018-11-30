package phpv

type ZClassType int
type ZClassAttr int
type ZObjectAttr int

const (
	// would use 1 << iota but those values come from php, so making them constants is more appropriate
	ZClassStatic           ZClassAttr = 0x001
	ZClassAbstract                    = 0x002
	ZClassImplAbstract                = 0x008 // an abstract method which has been implemented
	ZClassImplicitAbstract            = 0x010 // for classes
	ZClassExplicitAbstract            = 0x020 // for classes
	ZClassFinal                       = 0x040 // class attribute (not method)
	ZClassTrait                       = 0x80
	ZClassAnon                        = 0x100
	ZClassAnonBound                   = 0x200
	ZClassInherited                   = 0x400

	ZClassTypeImplicitAbstract ZClassType = 0x10
	ZClassTypeExplicitAbstract            = 0x20
	ZClassTypeInterface                   = 0x40
	ZClassTypeTrait                       = 0x80
	ZClassTypeAnon                        = 0x100

	ZAttrStatic         ZObjectAttr = ZObjectAttr(ZClassStatic)
	ZAttrAbstract                   = ZObjectAttr(ZClassAbstract)
	ZAttrFinal                      = 0x004 // final method, not the same value as ZClassFinal
	ZAttrPublic                     = 0x100
	ZAttrProtected                  = 0x200
	ZAttrPrivate                    = 0x400
	ZAttrAccess                     = ZAttrPublic | ZAttrProtected | ZAttrPrivate
	ZAttrImplicitPublic             = 0x1000 // method without flag
	ZAttrCtor                       = 0x2000
	ZAttrDtor                       = 0x4000
	ZAttrUserArgInfo                = 0x80    // method flag used by Closure::__invoke()
	ZAttrAllowStatic                = 0x10000 // method flag (bc only), any method that has this flag can be used statically and non statically.
	ZAttrShadow                     = 0x20000 // shadow of parent's private method/property
	ZAttrDeprecated                 = 0x40000 // deprecation flag
	ZAttrClosure                    = 0x100000
	ZAttrFakeClosure                = 0x40
	ZAttrGenerator                  = 0x800000
	ZAttrViaTrampoline              = 0x200000           // call through user function trampoline. e.g. __call, __callstatic
	ZAttrViaHandler                 = ZAttrViaTrampoline // call through internal function handler. e.g. Closure::invoke()
	ZAttrVariadic                   = 0x1000000
	ZAttrReturnRef                  = 0x4000000
	ZAttrUseGuard                   = 0x1000000  // class has magic methods __get/__set/__unset/__isset that use guards
	ZAttrHasTypeHints               = 0x10000000 // function has typed arguments
	ZAttrHasReturnType              = 0x40000000 // Function has a return type (or class has such non-private function)
)

func (a ZClassAttr) Has(c ZClassAttr) bool {
	return a&c == c
}

func (a ZClassType) Has(c ZClassType) bool {
	return a&c == c
}

func (a ZClassType) IsInterface() bool {
	return a.Has(ZClassTypeInterface)
}

func (a ZObjectAttr) Has(c ZObjectAttr) bool {
	return a&c == c
}

func (a ZObjectAttr) IsStatic() bool {
	return a.Has(ZAttrStatic)
}

func (a ZObjectAttr) Access() ZObjectAttr {
	return a & ZAttrAccess
}

func (a ZObjectAttr) IsPublic() bool {
	return a.Access() == ZAttrPublic
}

func (a ZObjectAttr) IsProtected() bool {
	return a.Access() == ZAttrProtected
}

func (a ZObjectAttr) IsPrivate() bool {
	return a.Access() == ZAttrPrivate
}

func (a ZClassAttr) String() string {
	var r []byte
	if a.Has(ZClassAbstract) {
		r = append(r, []byte(" abstract")...)
	}
	if a.Has(ZClassFinal) {
		r = append(r, []byte(" final")...)
	}

	if r == nil {
		return ""
	}
	return string(r[1:])
}
