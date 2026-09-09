package cache

import "context"

type Noop struct{}

func (Noop) Get(context.Context, string) (string, Result) { return "", Miss }
func (Noop) Set(context.Context, string, string)          {}
func (Noop) SetNegative(context.Context, string)          {}

var _ Cache = Noop{}
