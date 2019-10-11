package validate

import (
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
)

// Validator performs validation on a value and returns an error and a warning.
type Validator interface {
	Validate(Context) (error, error)
}

// ValidatorFunc is an adapter for a function that implements the Validator interface.
type ValidatorFunc func(Context) (error, error)

// Validate implements the Validator interface.
func (f ValidatorFunc) Validate(ctx Context) (error, error) {
	return f(ctx)
}

// And combines the validators as a conjunction.
func And(validators ...Validator) Validator {
	switch len(validators) {
	case 0:
		return NoOpValidator{}
	case 1:
		return validators[0]
	}

	return ValidatorFunc(func(ctx Context) (error, error) {
		var errs []error
		var warnings []error
		for _, v := range validators {
			err, warning := v.Validate(ctx)
			if err != nil {
				errs = append(errs, err)
				if ctx.Options.StopOnError {
					break
				}
			}
			if warning != nil {
				warnings = append(warnings, warning)
				if ctx.Options.shouldStopOnWarnings() {
					break
				}
			}
		}

		return mergeWarningsAndErrors(warnings, errs, "and")
	})
}

// CustomMessage wraps a validator's error with a custom message.
func CustomMessage(validator Validator, msg string) Validator {
	return ValidatorFunc(func(ctx Context) (error, error) {
		err, warning := validator.Validate(ctx)
		if err != nil {
			switch err.(type) {
			case *Error:
				return newError(msg), warning
			default:
				return err, warning
			}
		}

		return nil, warning
	})
}

// Empty requires the length of a string, array, slice, or map to be 0.
func Empty() Validator {
	return ValidatorFunc(func(ctx Context) (error, error) {
		val := indirect(ctx.Value)
		switch val.Kind() {
		case reflect.String, reflect.Array, reflect.Slice, reflect.Map:
			if val.Len() != 0 {
				return newError("must be empty"), nil
			}
			return nil, nil
		case reflect.Ptr:
			return newError("must be empty"), nil
		default:
			return isEmptyAllowed(val.Type(), "empty", nil), nil
		}
	})
}

func isEmptyAllowed(t reflect.Type, name string, args []string) error {
	switch t.Kind() {
	case reflect.String, reflect.Array, reflect.Slice, reflect.Map:
		return nil
	case reflect.Ptr:
		return isEmptyAllowed(t.Elem(), name, args)
	default:
		return InvalidTagArgumentsError{Message: "only pointers to/or strings, arrays, slices, and maps are supported", ValidatorName: name, Args: args}
	}
}

// Equal requires the value to be equal to the specified value.
func Equal(other interface{}) Validator {
	return ValidatorFunc(func(ctx Context) (error, error) {
		val := indirect(ctx.Value)
		if val.Kind() == reflect.Ptr {
			return newErrorf("must be equal to %v", other), nil
		}

		r, err := cmp(val, reflect.ValueOf(other))
		if err != nil {
			return err, nil
		}

		if r != 0 {
			return newErrorf("must be equal to %v", other), nil
		}

		return nil, nil
	})
}

// Field wraps a validator in a field validator.
func Field(name string, validator Validator) Validator {
	return ValidatorFunc(func(ctx Context) (error, error) {
		if !ctx.Value.IsValid() {
			return nil, nil
		}

		val := ctx.Value.FieldByName(name)
		if !val.IsValid() {
			return UnknownFieldError{Type: ctx.Value.Type(), Name: name}, nil
		}

		fctx := ctx
		fctx.Parent = &ctx
		fctx.Value = val

		err, warning := validator.Validate(fctx)
		if err != nil {
			switch err.(type) {
			case *Error:
				sf, _ := ctx.Value.Type().FieldByName(name)
				return newErrorf("%q %v", sf.Name, err), warning
			default:
				return err, warning
			}
		}

		return nil, warning
	})
}

// GreaterThan requires the value to be greater than the specified value.
func GreaterThan(other interface{}) Validator {
	return ValidatorFunc(func(ctx Context) (error, error) {
		val := indirect(ctx.Value)
		if val.Kind() == reflect.Ptr {
			return newErrorf("must be greater than %v", other), nil
		}

		r, err := cmp(val, reflect.ValueOf(other))
		if err != nil {
			return err, nil
		}

		if r <= 0 {
			return newErrorf("must be greater than %v", other), nil
		}

		return nil, nil
	})
}

// GreaterThanOrEqual requires the value to be greater than or equal to the specified value.
func GreaterThanOrEqual(other interface{}) Validator {
	return ValidatorFunc(func(ctx Context) (error, error) {
		val := indirect(ctx.Value)
		if val.Kind() == reflect.Ptr {
			return newErrorf("must be greater than or equal to %v", other), nil
		}

		r, err := cmp(val, reflect.ValueOf(other))
		if err != nil {
			return err, nil
		}

		if r < 0 {
			return newErrorf("must be greater than or equal to %v", other), nil
		}

		return nil, nil
	})
}

// In requires a value to be one of the specified values.
func In(values ...interface{}) Validator {
	return ValidatorFunc(func(ctx Context) (error, error) {
		val := indirect(ctx.Value)
		if val.Kind() == reflect.Ptr {
			return newErrorf("must be one of %v", values), nil
		}

		for _, v := range values {
			r, err := cmp(val, reflect.ValueOf(v))
			if err != nil {
				return err, nil
			}

			if r == 0 {
				return nil, nil
			}
		}

		return newErrorf("must be one of %v", values), nil
	})
}

// Items validates that all the items of a slice, array.
func Items(validator Validator) Validator {
	return ValidatorFunc(func(ctx Context) (error, error) {
		var errs []error
		var warnings []error
		val := indirect(ctx.Value)
		switch val.Kind() {
		case reflect.Array, reflect.Slice:
			len := val.Len()
			for i := 0; i < len; i++ {
				item := val.Index(i)

				fctx := ctx
				fctx.Parent = &ctx
				fctx.Value = item

				err, warning := validator.Validate(fctx)
				if err != nil {
					switch err.(type) {
					case *Error:
						errs = append(errs, newErrorf("[%d] %v", i, err))
						if ctx.Options.StopOnError {
							break
						}
					default:
						return err, warning
					}
				}
				if warning != nil {
					warnings = append(warnings, newErrorf("[%d] %v", i, warning))
					if ctx.Options.shouldStopOnWarnings() {
						break
					}
				}
			}
		case reflect.Map:
			mi := val.MapRange()
			for mi.Next() {
				item := mi.Value()

				fctx := ctx
				fctx.Parent = &ctx
				fctx.Value = item

				err, warning := validator.Validate(fctx)
				if err != nil {
					switch err.(type) {
					case *Error:
						errs = append(errs, newErrorf("[%v] %v", mi.Key(), err))
						if ctx.Options.StopOnError {
							break
						}
					default:
						return err, warning
					}
				}
				if warning != nil {
					warnings = append(warnings, newErrorf("[%v] %v", mi.Key(), warning))
					if ctx.Options.shouldStopOnWarnings() {
						break
					}
				}
			}
		default:
			return isItemsAllowed(ctx.Value.Type(), "items", nil), nil
		}

		return mergeWarningsAndErrors(warnings, errs, "and")
	})
}

func isItemsAllowed(t reflect.Type, name string, args []string) error {
	switch t.Kind() {
	case reflect.Array, reflect.Slice, reflect.Map:
		return nil
	case reflect.Ptr:
		return isItemsAllowed(t.Elem(), name, args)
	default:
		return InvalidTagArgumentsError{Message: "only pointers to/or arrays, slices, and maps are supported", ValidatorName: name, Args: args}
	}
}

// Length requires the length of a string, array, slice, or map to be exactly len.
func Length(len int) Validator {
	return ValidatorFunc(func(ctx Context) (error, error) {
		val := indirect(ctx.Value)
		switch val.Kind() {
		case reflect.String, reflect.Array, reflect.Slice, reflect.Map:
			if val.Len() != len {
				return newErrorf("must be of length %d", len), nil
			}
			return nil, nil
		case reflect.Ptr:
			return newErrorf("must be of length %d", len), nil
		default:
			return isLengthAllowed(val.Type(), "length", nil), nil
		}
	})
}

func isLengthAllowed(t reflect.Type, name string, args []string) error {
	switch t.Kind() {
	case reflect.String, reflect.Array, reflect.Slice, reflect.Map:
	case reflect.Ptr:
		return isLengthAllowed(t.Elem(), name, args)
	default:
		return InvalidTagArgumentsError{Message: "only pointers to/or strings, arrays, slices, and maps are supported", ValidatorName: name, Args: args}
	}

	if len(args) != 1 {
		return InvalidTagArgumentsError{Message: "1 argument is required", ValidatorName: name, Args: args}
	}

	_, err := strconv.Atoi(args[0])
	if err != nil {
		return InvalidTagArgumentsError{Message: "argument must be an integer", ValidatorName: name, Args: args}
	}

	return nil
}

// LessThan requires the value to be less than the specified value.
func LessThan(other interface{}) Validator {
	return ValidatorFunc(func(ctx Context) (error, error) {
		val := indirect(ctx.Value)
		if val.Kind() == reflect.Ptr {
			return newErrorf("must be less than %v", other), nil
		}

		r, err := cmp(val, reflect.ValueOf(other))
		if err != nil {
			return err, nil
		}

		if r >= 0 {
			return newErrorf("must be less than %v", other), nil
		}

		return nil, nil
	})
}

// LessThanOrEqual requires the value to be less than or equal to the specified value.
func LessThanOrEqual(other interface{}) Validator {
	return ValidatorFunc(func(ctx Context) (error, error) {
		val := indirect(ctx.Value)
		if val.Kind() == reflect.Ptr {
			return newErrorf("must be less than or equal to %v", other), nil
		}

		r, err := cmp(val, reflect.ValueOf(other))
		if err != nil {
			return err, nil
		}

		if r > 0 {
			return newErrorf("must be less than or equal to %v", other), nil
		}

		return nil, nil
	})
}

// MaxLength requires the length of a string, array, slice, or map to be less than or equal to len.
func MaxLength(len int) Validator {
	return ValidatorFunc(func(ctx Context) (error, error) {
		val := indirect(ctx.Value)
		switch val.Kind() {
		case reflect.String, reflect.Array, reflect.Slice, reflect.Map:
			if val.Len() > len {
				return newErrorf("must have max length %d", len), nil
			}
			return nil, nil
		case reflect.Ptr:
			return newErrorf("must have max length %d", len), nil
		default:
			return isLengthAllowed(val.Type(), "maxlength", nil), nil
		}
	})
}

// MinLength requires the length of a string, array, slice, or map to be greater than or equal to len.
func MinLength(len int) Validator {
	return ValidatorFunc(func(ctx Context) (error, error) {
		val := indirect(ctx.Value)
		switch val.Kind() {
		case reflect.String, reflect.Array, reflect.Slice, reflect.Map:
			if val.Len() < len {
				return newErrorf("must have min length %d", len), nil
			}
			return nil, nil
		case reflect.Ptr:
			return newErrorf("must have min length %d", len), nil
		default:
			return isLengthAllowed(val.Type(), "minlength", nil), nil
		}
	})
}

// Nil requires the value of any type that can be a pointer to be nil.
func Nil() Validator {
	return ValidatorFunc(func(ctx Context) (error, error) {
		switch ctx.Value.Kind() {
		case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice, reflect.UnsafePointer:
			if !ctx.Value.IsNil() {
				return newError("must be nil"), nil
			}
			return nil, nil
		default:
			return isNilAllowed(ctx.Value.Type(), "nil", nil), nil
		}
	})
}

func isNilAllowed(t reflect.Type, name string, args []string) error {
	switch t.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice, reflect.UnsafePointer:
		return nil
	default:
		return InvalidTagArgumentsError{Message: "only nil-able types are supported", ValidatorName: name, Args: args}
	}
}

// NoOpValidator is a validator that does nothing.
type NoOpValidator struct{}

// Validate implements the Validator interface.
func (NoOpValidator) Validate(Context) (error, error) {
	return nil, nil
}

// NotEmpty requires the length of a string, array, slice, or map to be greater than 0.
func NotEmpty() Validator {
	return ValidatorFunc(func(ctx Context) (error, error) {
		val := indirect(ctx.Value)
		switch val.Kind() {
		case reflect.String, reflect.Array, reflect.Slice, reflect.Map:
			if val.Len() == 0 {
				return newError("must not be empty"), nil
			}
			return nil, nil
		case reflect.Ptr:
			return newError("must not be empty"), nil
		default:
			return isNotEmptyAllowed(val.Type(), "notempty", nil), nil
		}
	})
}

func isNotEmptyAllowed(t reflect.Type, name string, args []string) error {
	switch t.Kind() {
	case reflect.String, reflect.Array, reflect.Slice, reflect.Map:
		return nil
	case reflect.Ptr:
		return isNotEmptyAllowed(t.Elem(), name, args)
	default:
		return InvalidTagArgumentsError{Message: "only pointers to/or strings, arrays, slices, and maps are supported", ValidatorName: name, Args: args}
	}
}

// NotEqual requires the value to not be equal to the specified value.
func NotEqual(other interface{}) Validator {
	return ValidatorFunc(func(ctx Context) (error, error) {
		val := indirect(ctx.Value)
		if val.Kind() == reflect.Ptr {
			return newErrorf("must not be equal to %v", other), nil
		}

		r, err := cmp(val, reflect.ValueOf(other))
		if err != nil {
			return err, nil
		}

		if r == 0 {
			return newErrorf("must not be equal to %v", other), nil
		}

		return nil, nil
	})
}

// NotNil requires the value of any type that can be a pointer to not be nil.
func NotNil() Validator {
	return ValidatorFunc(func(ctx Context) (error, error) {
		switch ctx.Value.Kind() {
		case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice, reflect.UnsafePointer:
			if ctx.Value.IsNil() {
				return newError("must not be nil"), nil
			}
			return nil, nil
		default:
			return isNotNilAllowed(ctx.Value.Type(), "notnil", nil), nil
		}
	})
}

func isNotNilAllowed(t reflect.Type, name string, args []string) error {
	switch t.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice, reflect.UnsafePointer:
		return nil
	default:
		return InvalidTagArgumentsError{Message: "only nil-able types are supported", ValidatorName: name, Args: args}
	}
}

// Or combines the validators as a disjunction.
func Or(validators ...Validator) Validator {
	switch len(validators) {
	case 0:
		return NoOpValidator{}
	case 1:
		return validators[0]
	}

	return ValidatorFunc(func(ctx Context) (error, error) {
		var errs []error
		var warnings []error
		for _, v := range validators {
			err, warning := v.Validate(ctx)
			if err == nil && warning == nil {
				return nil, nil
			}
			if err != nil {
				errs = append(errs, err)
			}
			if warning != nil {
				warnings = append(warnings, warning)
			}
		}

		return mergeWarningsAndErrors(warnings, errs, "or")
	})
}

// Zero requires the value to be the zero value.
func Zero() Validator {
	return ValidatorFunc(func(ctx Context) (error, error) {
		if !isZero(ctx.Value) {
			return newErrorf("must be \"%v\"", zeroValue(ctx.Value.Type())), nil
		}
		return nil, nil
	})
}

func zeroValue(t reflect.Type) interface{} {
	return reflect.Zero(t).Interface()
}

func isZero(rval reflect.Value) bool {
	switch rval.Kind() {
	case reflect.Bool:
		return !rval.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rval.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return rval.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return math.Float64bits(rval.Float()) == 0
	case reflect.Complex64, reflect.Complex128:
		c := rval.Complex()
		return math.Float64bits(real(c)) == 0 && math.Float64bits(imag(c)) == 0
	case reflect.Array:
		for i := 0; i < rval.Len(); i++ {
			if !isZero(rval.Index(i)) {
				return false
			}
		}
		return true
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice, reflect.UnsafePointer:
		return rval.IsNil()
	case reflect.String:
		return rval.Len() == 0
	case reflect.Struct:
		for i := 0; i < rval.NumField(); i++ {
			if !isZero(rval.Field(i)) {
				return false
			}
		}
		return true
	default:
		// This should never happens, but will act as a safeguard for
		// later, as a default value doesn't makes sense here.
		panic(fmt.Sprintf("reflect.Value.IsZero (%s)", rval.Kind()))
	}
}

func isCmpAllowed(t reflect.Type, name string, args []string) error {
	if len(args) != 1 {
		return InvalidTagArgumentsError{Message: "1 argument is required", ValidatorName: name, Args: args}
	}

	switch t.Kind() {
	case reflect.Bool:
		_, err := tryParseString(t, args[0])
		if err != nil {
			return InvalidTagArgumentsError{Message: "argument must be a boolean", ValidatorName: name, Args: args}
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		_, err := tryParseString(t, args[0])
		if err != nil {
			return InvalidTagArgumentsError{Message: "argument must be an integer", ValidatorName: name, Args: args}
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		_, err := tryParseString(t, args[0])
		if err != nil {
			return InvalidTagArgumentsError{Message: "argument must be an unsigned integer", ValidatorName: name, Args: args}
		}
	case reflect.Float32, reflect.Float64:
		_, err := tryParseString(t, args[0])
		if err != nil {
			return InvalidTagArgumentsError{Message: "argument must be a floating-point number", ValidatorName: name, Args: args}
		}
	case reflect.Ptr:
		return isCmpAllowed(t.Elem(), name, args)
	case reflect.String:
	default:
		return InvalidTagArgumentsError{Message: "only pointers to/or strings, bools, and numbers are allowed", ValidatorName: name, Args: args}
	}

	return nil
}

func cmp(val reflect.Value, other reflect.Value) (int, error) {
	switch val.Kind() {
	case reflect.Bool:
		switch other.Kind() {
		case reflect.Bool:
			if val.Bool() == other.Bool() {
				return 0, nil
			}
			return -1, nil
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v := val.Int()
		switch other.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			o := other.Int()
			if v < o {
				return -1, nil
			} else if v == o {
				return 0, nil
			} else {
				return 1, nil
			}
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			o := other.Uint()
			if v < 0 {
				return -1, nil
			}
			if uint64(v) < 0 {
				return -1, nil
			} else if uint64(v) == o {
				return 0, nil
			} else {
				return 1, nil
			}
		case reflect.Float32, reflect.Float64:
			f := float64(v)
			o := other.Float()
			if f < o {
				return -1, nil
			} else if f == o {
				return 0, nil
			} else {
				return 1, nil
			}
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		v := val.Uint()
		switch other.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			o := other.Int()
			if o < 0 {
				return 1, nil
			}
			if v < uint64(0) {
				return -1, nil
			} else if v == uint64(o) {
				return 0, nil
			} else {
				return 1, nil
			}
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			o := other.Uint()
			if v < o {
				return -1, nil
			} else if v == o {
				return 0, nil
			} else {
				return 1, nil
			}
		case reflect.Float32, reflect.Float64:
			f := float64(v)
			o := other.Float()
			if f < o {
				return -1, nil
			} else if f == o {
				return 0, nil
			} else {
				return 1, nil
			}
		}
	case reflect.String:
		v := val.String()
		switch other.Kind() {
		case reflect.String:
			o := other.String()
			return strings.Compare(v, o), nil
		}
	}

	return 0, fmt.Errorf("incompatible types for comparision: %s and %s", val.Type(), other.Type())
}

func tryParseString(t reflect.Type, arg string) (interface{}, error) {
	switch t.Kind() {
	case reflect.Bool:
		v, err := strconv.ParseBool(arg)
		if err != nil {
			return nil, err
		}
		return v, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v, err := strconv.ParseInt(arg, 10, 64)
		if err != nil {
			return nil, err
		}
		return v, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		v, err := strconv.ParseUint(arg, 10, 64)
		if err != nil {
			return nil, err
		}
		return v, nil
	case reflect.Float32, reflect.Float64:
		v, err := strconv.ParseFloat(arg, 64)
		if err != nil {
			return nil, err
		}
		return v, nil
	case reflect.Ptr:
		return tryParseString(t.Elem(), arg)
	case reflect.String:
		return arg, nil
	default:
		return nil, newError("unsupported type")
	}
}

func indirect(val reflect.Value) reflect.Value {
	for val.Kind() == reflect.Ptr {
		if val.IsNil() {
			break
		}
		val = val.Elem()
	}

	return val
}
