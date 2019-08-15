package validate

import (
	"reflect"
	"sync"
)

// NewRegistryBuilder makes a RegistryBuilder.
func NewRegistryBuilder() *RegistryBuilder {
	return &RegistryBuilder{
		structTagName:         DefaultStructTagName,
		structTagParser:       DefaultStructTagParser,
		validators:            make(map[reflect.Type]Validator),
		tagValidatorFactories: make(map[string]TagValidatorFactory),
	}
}

// RegistryBuilder builds a Registry.
type RegistryBuilder struct {
	structTagName         string
	structTagParser       StructTagParser
	validators            map[reflect.Type]Validator
	tagValidatorFactories map[string]TagValidatorFactory
}

// Build the registry.
func (rb *RegistryBuilder) Build() *Registry {
	r := Registry{
		structTagName:         rb.structTagName,
		structTagParser:       rb.structTagParser,
		validators:            make(map[reflect.Type]Validator),
		tagValidatorFactories: make(map[string]TagValidatorFactory),
	}

	for t, v := range rb.validators {
		r.validators[t] = v
	}

	for t, vf := range rb.tagValidatorFactories {
		r.tagValidatorFactories[t] = vf
	}

	return &r
}

// RegisterValidator registers a Validator for the specific type.
func (rb *RegistryBuilder) RegisterValidator(t reflect.Type, v Validator) *RegistryBuilder {
	rb.validators[t] = v
	return rb
}

// RegisterTagValidatorFactory registers a TagValidatorFactory for the specified name.
func (rb *RegistryBuilder) RegisterTagValidatorFactory(name string, vf TagValidatorFactory) *RegistryBuilder {
	rb.tagValidatorFactories[name] = vf
	return rb
}

// SetStructTagName sets the tag name to use when building validators from struct tags.
func (rb *RegistryBuilder) SetStructTagName(name string) *RegistryBuilder {
	rb.structTagName = name
	return rb
}

// SetStructTagParser sets the parser for struct tags.
func (rb *RegistryBuilder) SetStructTagParser(stp StructTagParser) *RegistryBuilder {
	rb.structTagParser = stp
	return rb
}

// Registry holds validators for types.
type Registry struct {
	structTagName         string
	structTagParser       StructTagParser
	validators            map[reflect.Type]Validator
	tagValidatorFactories map[string]TagValidatorFactory

	lock sync.RWMutex
}

// LookupTagValidatorFactory will inspect the registry for a ValidatorFactory
// of the specified name.
func (r *Registry) LookupTagValidatorFactory(name string) (TagValidatorFactory, error) {
	vf, ok := r.tagValidatorFactories[name]
	if !ok {
		return nil, ErrNoTagValidatorFactory{name}
	}

	return vf, nil
}

// LookupValidator will inspect the registry for a Validator for
// the type provided. If no validator is found, an error will be returned.
func (r *Registry) LookupValidator(t reflect.Type) (Validator, error) {
	ctx := ResolutionContext{
		structTagParser: r.structTagParser,
		Type:            t,
		registry:        r,
	}

	return r.lookupValidator(ctx)
}

var tValidator = reflect.TypeOf((*Validator)(nil)).Elem()

func (r *Registry) lookupValidator(ctx ResolutionContext) (Validator, error) {
	r.lock.RLock()
	v, ok := r.validators[ctx.Type]
	r.lock.RUnlock()
	if ok {
		if v == nil {
			return nil, ErrNoValidator{ctx.Type}
		}

		return v, nil
	}

	var err error
	v, err = buildValidator(ctx)
	if err != nil {
		return nil, err
	}

	r.lock.Lock()
	r.validators[ctx.Type] = v
	r.lock.Unlock()
	return v, nil
}
