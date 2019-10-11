package validate

import (
	"reflect"
)

// Validate performs validation on the obj and returns an error and a warning if it existed, respectively.
//
// Example:
//   if err := validate.Validate(MyObject{}); err != nil {
//   	// handle validation error
//   }
func Validate(obj interface{}, options ...Option) (error, error) {
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
			return err, nil
		}
	}

	ctx := Context{
		Options: opts,
		Value:   rval,
	}

	if ctx.Options.WarningsAsErrors {
		return mergeWarning(validator.Validate(ctx)), nil
	}
	return validator.Validate(ctx)
}

// ValidateWarningsAsErrors is the same as Validate(), but returns warnings as
// errors. This is a convenience function for callers that do not care to
// distinguish between an error and a warning and therefore do not want to deal
// two returns.
func ValidateWarningsAsErrors(obj interface{}, options ...Option) error {
	return mergeWarning(Validate(obj, options...))
}
