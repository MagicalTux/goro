package spl

import (
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// restoreMemberProperties restores member properties from a serialized array
// during __unserialize. Handles mangled property names with NUL bytes:
// - \0*\0name -> protected property
// - \0ClassName\0name -> private property
// - name -> public/dynamic property
func restoreMemberProperties(ctx phpv.Context, o *phpobj.ZObject, memberArr *phpv.ZArray) {
	for k, v := range memberArr.Iterate(ctx) {
		key := string(k.AsString(ctx))
		if len(key) > 0 && key[0] == 0 {
			// Mangled name - set directly on hash table
			// \0*\0name = protected, \0ClassName\0name = private
			if len(key) > 2 && key[1] == '*' && key[2] == 0 {
				// Protected property: \0*\0name -> set as bare name
				propName := phpv.ZString(key[3:])
				o.HashTable().SetString(propName, v)
			} else {
				// Private property: \0ClassName\0name
				idx := strings.IndexByte(key[1:], 0)
				if idx >= 0 {
					propName := phpv.ZString(key[idx+2:])
					// Store with the mangled key in the hash table
					// so var_dump shows the correct visibility
					storageKey := "*" + key[1:idx+1] + ":" + key[idx+2:]
					o.HashTable().SetString(phpv.ZString(storageKey), v)
					// Also try to set on the declared property
					o.HashTable().SetString(propName, v)
				} else {
					// Malformed - just set as-is
					o.HashTable().SetString(phpv.ZString(key), v)
				}
			}
		} else {
			// Public/dynamic property - use ObjectSet
			o.ObjectSet(ctx, k, v)
		}
	}
}

func init() {
	initArrayIterator()
	initArrayObject()
	initInfiniteIterator()
	initFileInfo()
	initSplFileObject()
	initDirectoryIterator()
	initSplFixedArray()
	initSplDoublyLinkedList()
	initObjectStorage()
	initPriorityQueue()
	initIteratorIterator()
	initLimitIterator()
	initCachingIterator()
	initAppendIterator()
	initRegexIterator()
	initRecursiveArrayIterator()
	initRecursiveIteratorIterator()
	initNoRewindIterator()
	initEmptyIterator()
	initCallbackFilterIterator()
	initFilterIterator()
	initRecursiveFilterIterator()
	initRecursiveCachingIterator()
	initRecursiveRegexIterator()
	initRecursiveCallbackFilterIterator()
	initParentIterator()
	initMultipleIterator()
	initSplHeap()
	initRecursiveDirectoryIterator()
	initRecursiveTreeIterator()

	phpctx.RegisterExt(&phpctx.Ext{
		Name:    "SPL",
		Version: core.VERSION,
		Classes: []*phpobj.ZClass{
			Countable,
			OuterIterator,
			RecursiveIterator,
			SeekableIterator,
			SplObserver,
			SplSubject,
			ArrayIteratorClass,
			ArrayObjectClass,
			InfiniteIteratorClass,
			SplFileInfoClass,
			SplFileObjectClass,
			SplTempFileObjectClass,
			DirectoryIteratorClass,
			FilesystemIteratorClass,
			SplFixedArrayClass,
			SplDoublyLinkedListClass,
			SplStackClass,
			SplQueueClass,
			SplObjectStorageClass,
			SplPriorityQueueClass,
			IteratorIteratorClass,
			LimitIteratorClass,
			CachingIteratorClass,
			AppendIteratorClass,
			RegexIteratorClass,
			RecursiveArrayIteratorClass,
			RecursiveIteratorIteratorClass,
			NoRewindIteratorClass,
			EmptyIteratorClass,
			CallbackFilterIteratorClass,
			FilterIteratorClass,
			RecursiveFilterIteratorClass,
			RecursiveCachingIteratorClass,
			RecursiveRegexIteratorClass,
			RecursiveCallbackFilterIteratorClass,
			ParentIteratorClass,
			MultipleIteratorClass,
			SplHeapClass,
			SplMinHeapClass,
			SplMaxHeapClass,
			RecursiveDirectoryIteratorClass,
			RecursiveTreeIteratorClass,
			GlobIteratorClass,
		},
		Functions: map[string]*phpctx.ExtFunction{
			"spl_object_hash":        {Func: splObjectHash, Args: []*phpctx.ExtFunctionArg{}},
			"spl_object_id":          {Func: splObjectId, Args: []*phpctx.ExtFunctionArg{}},
			"iterator_to_array":      {Func: iteratorToArray, Args: []*phpctx.ExtFunctionArg{}},
			"iterator_count":         {Func: iteratorCount, Args: []*phpctx.ExtFunctionArg{}},
			"iterator_apply":         {Func: iteratorApply, Args: []*phpctx.ExtFunctionArg{}},
			"class_implements":       {Func: classImplements, Args: []*phpctx.ExtFunctionArg{}},
			"class_parents":          {Func: classParents, Args: []*phpctx.ExtFunctionArg{}},
			"class_uses":             {Func: classUses, Args: []*phpctx.ExtFunctionArg{}},
			"spl_autoload_functions":  {Func: splAutoloadFunctions, Args: []*phpctx.ExtFunctionArg{}},
			"spl_autoload_call":      {Func: splAutoloadCall, Args: []*phpctx.ExtFunctionArg{}},
			"spl_autoload_extensions": {Func: splAutoloadExtensions, Args: []*phpctx.ExtFunctionArg{}},
			"spl_classes":            {Func: splClasses, Args: []*phpctx.ExtFunctionArg{}},
		},
		Constants: map[phpv.ZString]phpv.Val{},
	})
}
