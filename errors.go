package validate

import (
	"fmt"
	"reflect"
)

func mergeWarning(err error, warning error) error {
	if err == nil && warning == nil {
		return nil
	}

	errs := []error{}
	if err != nil {
		errs = append(errs, err)
	}
	if warning != nil {
		errs = append(errs, warning)
	}
	return newError(mergeErrorMessages(errs, "and"))
}

func mergeErrorMessages(errs []error, sep string) string {
	paddedSep := fmt.Sprintf(" %s ", sep)
	if len(errs) == 0 {
		panic("must provide at least one error")
	}
	errmsg := errs[0].Error()
	for i := 1; i < len(errs); i++ {
		errmsg += paddedSep + errs[i].Error()
	}

	return errmsg
}

func mergeIntoWarningsAndErrors(warnings []error, errs []error, sep string) (error, error) {
	var warning error
	if len(warnings) > 0 {
		warning = newError(mergeErrorMessages(warnings, sep))
	}

	if len(errs) > 0 {
		return newError(mergeErrorMessages(errs, sep)), warning
	}

	return nil, warning
}

func newError(msg string) *Error {
	return &Error{Message: msg}
}

func newErrorf(msg string, args ...interface{}) *Error {
	return &Error{Message: fmt.Sprintf(msg, args...)}
}

// Error is a validation error.
type Error struct {
	Message string
}

// Error implements the error interface.
func (e Error) Error() string {
	return e.Message
}

// InvalidTagArgumentsError is returned when a tag validator was provided with invalid arguments.
type InvalidTagArgumentsError struct {
	Message       string
	ValidatorName string
	Args          []string
}

// Error implements the error interface.
func (e InvalidTagArgumentsError) Error() string {
	return "(" + e.ValidatorName + ") " + e.Message
}

// UnknownFieldError is returned a when a field is invalid.
type UnknownFieldError struct {
	Type reflect.Type
	Name string
}

// Error implements the error interface.
func (e UnknownFieldError) Error() string {
	return fmt.Sprintf("%q does not contain a field %q", e.Type.String(), e.Name)
}

// TODO: rename the below

// ErrNoValidator is returned when there wasn't a validator available for a type.
type ErrNoValidator struct {
	Type reflect.Type
}

// Error implements the error interface.
func (e ErrNoValidator) Error() string {
	if e.Type == nil {
		return "no validator found for <nil>"
	}
	return "no validator found for " + e.Type.String()
}

// ErrNoTagValidatorFactory is returned when there wasn't a TagValidatorFactory available for a name.
type ErrNoTagValidatorFactory struct {
	Name string
}

// Error implements the error interface.
func (e ErrNoTagValidatorFactory) Error() string {
	return "no validator factory found for " + e.Name
}

// ErrInvalidType is returned when a validator is applied to an unsupported type.
type ErrInvalidType struct {
	Type          reflect.Type
	ValidatorName string
}

// Error implements the error interface.
func (e ErrInvalidType) Error() string {
	return "validator " + e.ValidatorName + " cannot be applied to " + e.Type.String()
}
