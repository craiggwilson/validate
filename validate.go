package validate

import "reflect"

// Validate performs validation on the obj and returns an error if one existed.
//
// Example:
//   if err := validate.Validate(MyObject{}); err != nil {
//   	// handle validation error
//   }
func Validate(obj interface{}, options ...Option) error {
	opts := defaultOptions()
	for _, option := range options {
		option(opts)
	}

	rval := reflect.ValueOf(obj)

	validator := opts.Validator
	if validator == nil {
		var err error
		validator, err = opts.Registry.LookupValidator(rval.Type())
		if err != nil {
			return err
		}
	}

	ctx := Context{
		Options:  opts,
		Value: rval,
	}

	return validator.Validate(ctx)
}
