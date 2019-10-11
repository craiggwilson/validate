package validate

import (
	"strconv"
)

// TagValidatorFactory creates validators based on a name and arguments.
type TagValidatorFactory interface {
	Create(ResolutionContext, string, []string) (Validator, error)
}

// TagValidatorFactoryFunc is an adapter for a function that implements the TagValidatorFactory interface.
type TagValidatorFactoryFunc func(ResolutionContext, string, []string) (Validator, error)

// Create implements the TagValidatorFactory interface.
func (f TagValidatorFactoryFunc) Create(ctx ResolutionContext, name string, args []string) (Validator, error) {
	return f(ctx, name, args)
}

// RegisterDefaultTagValidatorFactories registers all the default validators into the RegistryBuilder.
func RegisterDefaultTagValidatorFactories(rb *RegistryBuilder) *RegistryBuilder {
	rb.RegisterTagValidatorFactory("empty", TagValidatorFactoryFunc(EmptyFactory))
	rb.RegisterTagValidatorFactory("eq", TagValidatorFactoryFunc(EqualFactory))
	rb.RegisterTagValidatorFactory("gt", TagValidatorFactoryFunc(GreaterThanFactory))
	rb.RegisterTagValidatorFactory("gte", TagValidatorFactoryFunc(GreaterThanOrEqualFactory))
	rb.RegisterTagValidatorFactory("in", TagValidatorFactoryFunc(InFactory))
	rb.RegisterTagValidatorFactory("items", TagValidatorFactoryFunc(ItemsFactory))
	rb.RegisterTagValidatorFactory("len", TagValidatorFactoryFunc(LengthFactory))
	rb.RegisterTagValidatorFactory("lt", TagValidatorFactoryFunc(LessThanFactory))
	rb.RegisterTagValidatorFactory("lte", TagValidatorFactoryFunc(LessThanOrEqualFactory))
	rb.RegisterTagValidatorFactory("maxlen", TagValidatorFactoryFunc(MaxLengthFactory))
	rb.RegisterTagValidatorFactory("minlen", TagValidatorFactoryFunc(MinLengthFactory))
	rb.RegisterTagValidatorFactory("neq", TagValidatorFactoryFunc(NotEqualFactory))
	rb.RegisterTagValidatorFactory("nil", TagValidatorFactoryFunc(NilFactory))
	rb.RegisterTagValidatorFactory("notempty", TagValidatorFactoryFunc(NotEmptyFactory))
	rb.RegisterTagValidatorFactory("notnil", TagValidatorFactoryFunc(NotNilFactory))
	rb.RegisterTagValidatorFactory("notzero", TagValidatorFactoryFunc(NotZeroFactory))
	rb.RegisterTagValidatorFactory("struct", TagValidatorFactoryFunc(StructFactory))
	rb.RegisterTagValidatorFactory("zero", TagValidatorFactoryFunc(ZeroFactory))

	return rb
}

// EmptyFactory generates a Validator that requires the length of a string, array, slice, or map to be 0.
func EmptyFactory(ctx ResolutionContext, name string, args []string) (Validator, error) {
	err := isEmptyAllowed(ctx.Type, name, args)
	if err != nil {
		return nil, err
	}

	return Empty(), nil
}

// EqualFactory generates a Validator that requires a value to be equal to a specified other.
func EqualFactory(ctx ResolutionContext, name string, args []string) (Validator, error) {
	err := isCmpAllowed(ctx.Type, name, args)
	if err != nil {
		return nil, err
	}

	v, _ := tryParseString(ctx.Type, args[0])

	return Equal(v), nil
}

// GreaterThanFactory generates a Validator that requires a value to be greater than a specified other.
func GreaterThanFactory(ctx ResolutionContext, name string, args []string) (Validator, error) {
	err := isCmpAllowed(ctx.Type, name, args)
	if err != nil {
		return nil, err
	}

	v, _ := tryParseString(ctx.Type, args[0])

	return GreaterThan(v), nil
}

// GreaterThanOrEqualFactory generates a Validator that requires a value to be greater than or equal to a specified other.
func GreaterThanOrEqualFactory(ctx ResolutionContext, name string, args []string) (Validator, error) {
	err := isCmpAllowed(ctx.Type, name, args)
	if err != nil {
		return nil, err
	}

	v, _ := tryParseString(ctx.Type, args[0])

	return GreaterThanOrEqual(v), nil
}

// InFactory generates a Validator that requires a value to be on of a listof specified values.
func InFactory(ctx ResolutionContext, name string, args []string) (Validator, error) {
	var vs []interface{}
	for i := 0; i < len(args); i++ {
		err := isCmpAllowed(ctx.Type, name, args[i:i+1])
		if err != nil {
			return nil, err
		}
		v, _ := tryParseString(ctx.Type, args[i])
		vs = append(vs, v)
	}

	return In(vs...), nil
}

// ItemsFactory generates a Validator for items in a slice or array or values of a map.
func ItemsFactory(ctx ResolutionContext, name string, args []string) (Validator, error) {
	err := isItemsAllowed(ctx.Type, name, args)
	if err != nil {
		return nil, err
	}

	itemsTagName := "validateItems"
	if len(args) == 1 {
		itemsTagName = args[0]
	}

	itemType := ctx.Type.Elem()
	ctx.Type = itemType

	stpr, err := ctx.ParseStructTags(itemsTagName)
	if err != nil {
		return nil, err
	}

	validator := stpr.Validator
	if validator == nil {
		return NoOpValidator{}, nil
	}
	if stpr.CustomMessage != "" {
		validator = CustomMessage(validator, stpr.CustomMessage)
	}

	return Items(validator), nil
}

// LengthFactory generates a Validator that requires the length of a string, array, slice, or map to be of a specified length.
func LengthFactory(ctx ResolutionContext, name string, args []string) (Validator, error) {
	err := isLengthAllowed(ctx.Type, name, args)
	if err != nil {
		return nil, err
	}

	requiredLen, _ := strconv.Atoi(args[0])
	return Length(requiredLen), nil
}

// LessThanFactory generates a Validator that requires a value to be less than a specified other.
func LessThanFactory(ctx ResolutionContext, name string, args []string) (Validator, error) {
	err := isCmpAllowed(ctx.Type, name, args)
	if err != nil {
		return nil, err
	}

	v, _ := tryParseString(ctx.Type, args[0])

	return LessThan(v), nil
}

// LessThanOrEqualFactory generates a Validator that requires a value to be less than or equal to a specified other.
func LessThanOrEqualFactory(ctx ResolutionContext, name string, args []string) (Validator, error) {
	err := isCmpAllowed(ctx.Type, name, args)
	if err != nil {
		return nil, err
	}

	v, _ := tryParseString(ctx.Type, args[0])

	return LessThanOrEqual(v), nil
}

// MaxLengthFactory generates a Validator that requires the length of a string, array, slice, or map to be less than or equal to a specified length.
func MaxLengthFactory(ctx ResolutionContext, name string, args []string) (Validator, error) {
	err := isLengthAllowed(ctx.Type, name, args)
	if err != nil {
		return nil, err
	}

	requiredLen, _ := strconv.Atoi(args[0])
	return MaxLength(requiredLen), nil
}

// MinLengthFactory generates a Validator that requires the length of a string, array, slice, or map to be greater than or equal to a specified length.
func MinLengthFactory(ctx ResolutionContext, name string, args []string) (Validator, error) {
	err := isLengthAllowed(ctx.Type, name, args)
	if err != nil {
		return nil, err
	}

	requiredLen, _ := strconv.Atoi(args[0])
	return MinLength(requiredLen), nil
}

// NilFactory generates a Validator that requires the value to be nil.
func NilFactory(ctx ResolutionContext, name string, args []string) (Validator, error) {
	err := isNilAllowed(ctx.Type, name, args)
	if err != nil {
		return nil, err
	}

	return Nil(), nil
}

// NotEmptyFactory generates a Validator that requires the length of a string, array, slice, or map to be 0.
func NotEmptyFactory(ctx ResolutionContext, name string, args []string) (Validator, error) {
	err := isNotEmptyAllowed(ctx.Type, name, args)
	if err != nil {
		return nil, err
	}

	return NotEmpty(), nil
}

// NotEqualFactory generates a Validator that requires a value to not be equal to a specified other.
func NotEqualFactory(ctx ResolutionContext, name string, args []string) (Validator, error) {
	err := isCmpAllowed(ctx.Type, name, args)
	if err != nil {
		return nil, err
	}

	v, _ := tryParseString(ctx.Type, args[0])

	return NotEqual(v), nil
}

// NotNilFactory generates a Validator that requires the value to not be nil.
func NotNilFactory(ctx ResolutionContext, name string, args []string) (Validator, error) {
	err := isNotNilAllowed(ctx.Type, name, args)
	if err != nil {
		return nil, err
	}

	return NotNil(), nil
}

// NotZeroFactory generates a Validator that guards against a zero value.
func NotZeroFactory(ctx ResolutionContext, name string, args []string) (Validator, error) {
	if len(args) > 0 {
		return nil, InvalidTagArgumentsError{Message: "no arguments are allowed", ValidatorName: name, Args: args}
	}

	return ValidatorFunc(func(ctx Context) (error, error) {
		if isZero(ctx.Value) {
			return newErrorf("must not be \"%v\"", zeroValue(ctx.Value.Type())), nil
		}
		return nil, nil
	}), nil
}

// StructFactory generates a Validator for a struct.
func StructFactory(ctx ResolutionContext, name string, args []string) (Validator, error) {
	return ctx.LookupValidator(ctx.Type)
}

// ZeroFactory generates a Validator that requires the zero value.
func ZeroFactory(ctx ResolutionContext, name string, args []string) (Validator, error) {
	if len(args) > 0 {
		return nil, InvalidTagArgumentsError{Message: "no arguments are allowed", ValidatorName: name, Args: args}
	}

	return Zero(), nil
}
