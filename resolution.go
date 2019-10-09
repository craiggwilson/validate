package validate

import (
	"fmt"
	"reflect"
)

// ResolutionContext holds contextual information for resolving a validator.
type ResolutionContext struct {
	Parent      *ResolutionContext
	StructField reflect.StructField
	Type        reflect.Type

	registry        *Registry
	structTagParser StructTagParser
}

// LookupTagValidatorFactory will inspect the registry for a ValidatorFactory
// of the specified name.
func (ctx *ResolutionContext) LookupTagValidatorFactory(name string) (TagValidatorFactory, error) {
	return ctx.registry.LookupTagValidatorFactory(name)
}

// LookupValidator looks up the validator.
func (ctx *ResolutionContext) LookupValidator(t reflect.Type) (Validator, error) {
	parent := ctx.Parent
	for parent != nil {
		if parent.Type == t {
			return delayedLookup(ctx.registry, t), nil
		}
		parent = parent.Parent
	}

	cctx := *ctx
	cctx.Parent = ctx
	cctx.Type = t
	return ctx.registry.lookupValidator(cctx)
}

// ParseStructTags parses the struct tags for the given tag name.
func (ctx *ResolutionContext) ParseStructTags(tagName string) (*StructTagParseResult, error) {
	return ctx.structTagParser.ParseStructTags(*ctx, tagName)
}

func buildValidator(ctx ResolutionContext) (Validator, error) {
	// We want to consider only the base type T. If we get *T, then avoid adding
	// its Validator() implementation for now, and pass it to
	// buildNonImplValidator(), which will strip off the * and call back here.
	var validatorImpl Validator = NoOpValidator{}
	if ctx.Type.Implements(tValidator) && reflect.PtrTo(ctx.Type).Implements(tValidator) { // Copy receiver.
		validatorImpl = ValidatorFunc(func(ctx Context) error {
			v := ctx.Value.Interface().(Validator)
			return v.Validate(ctx)
		})
	} else if reflect.PtrTo(ctx.Type).Implements(tValidator) { // Pointer receiver.
		validatorImpl = ValidatorFunc(func(ctx Context) error {
			var ptr reflect.Value
			if ctx.Value.CanAddr() {
				ptr = ctx.Value.Addr()
			} else {
				ptr = reflect.New(ctx.Value.Type())
				ptr.Elem().Set(ctx.Value)
			}

			v := ptr.Interface().(Validator)
			return v.Validate(ctx)
		})
	}

	nonImplValidator, err := buildNonImplValidator(ctx)
	if err != nil {
		return nil, err
	}

	return And(validatorImpl, nonImplValidator), nil
}

func buildNonImplValidator(ctx ResolutionContext) (Validator, error) {
	switch ctx.Type.Kind() {
	case reflect.Struct:
		return buildStructValidator(ctx)
	case reflect.Ptr:
		// If we got *T, get T and have it hit buildValidator() eventually.
		v, err := ctx.LookupValidator(ctx.Type.Elem())
		if err != nil {
			return nil, err
		}
		return ValidatorFunc(func(ctx Context) error {
			pctx := ctx
			pctx.Parent = &ctx
			pctx.Value = ctx.Value.Elem()

			return v.Validate(pctx)
		}), nil
	case reflect.Interface:
		registry := ctx.registry
		return ValidatorFunc(func(ctx Context) error {
			val := ctx.Value.Elem()
			v, err := registry.LookupValidator(val.Type())
			if err != nil {
				return err
			}

			pctx := ctx
			pctx.Parent = &ctx
			pctx.Value = val

			return v.Validate(pctx)
		}), nil
	}

	return nil, ErrNoValidator{Type: ctx.Type}
}

// Struct builds a validator based on struct tags for the type.
func buildStructValidator(ctx ResolutionContext) (Validator, error) {
	if ctx.Type.Kind() != reflect.Struct {
		return nil, fmt.Errorf("cannot build a struct validator for kind %s", ctx.Type.Kind())
	}
	numFields := ctx.Type.NumField()
	var validators []Validator
	for i := 0; i < numFields; i++ {
		sf := ctx.Type.Field(i)

		cctx := ctx
		cctx.Parent = &ctx
		cctx.Type = sf.Type
		cctx.StructField = sf

		stpr, err := cctx.ParseStructTags(cctx.registry.structTagName)
		if err != nil {
			return nil, err
		}

		validator := stpr.Validator
		validator = Field(sf.Name, validator)

		if len(stpr.CustomMessage) > 0 {
			validator = CustomMessage(validator, stpr.CustomMessage)
		}

		validators = append(validators, validator)
	}

	return And(validators...), nil
}

func delayedLookup(r *Registry, t reflect.Type) Validator {
	var v Validator
	return ValidatorFunc(func(ctx Context) error {
		if v == nil {
			var err error
			v, err = r.LookupValidator(t)
			if err != nil {
				return err
			}
		}

		return v.Validate(ctx)
	})
}
