package spl

import (
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

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
			//"spl_autoload_extensions": {Func: splAutoloadExtensions, Args: []*phpctx.ExtFunctionArg{}},
			"spl_classes":            {Func: splClasses, Args: []*phpctx.ExtFunctionArg{}},
		},
		Constants: map[phpv.ZString]phpv.Val{},
	})
}
