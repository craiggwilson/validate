package validate

import "reflect"

// Context holds all the information required for validating an object.
type Context struct {
	Options  *Options

	Parent *Context
	Value  reflect.Value
}

