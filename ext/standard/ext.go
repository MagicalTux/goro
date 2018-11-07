package standard

import "git.atonline.com/tristantech/gophp/core"

func init() {
	core.RegisterExt(&core.Ext{
		Name: "standard",
		Functions: map[string]*core.ExtFunction{
			"echo": &core.ExtFunction{Func: stdFuncEcho, Args: []*core.ExtFunctionArg{&core.ExtFunctionArg{ArgName: "output"}, &core.ExtFunctionArg{ArgName: "...", Optional: true}}},
		},
	})
}
