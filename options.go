package validate

var (
	// DefaultRegistry is the default registry.
	DefaultRegistry = func() *Registry {
		rb := NewRegistryBuilder()
		RegisterDefaultTagValidatorFactories(rb)
		return rb.Build()
	}()
	// DefaultStopOnError is the default value for stopping validation upon encountering an error.
	DefaultStopOnError = false
)

func defaultOptions() *Options {
	return &Options{
		Registry:    DefaultRegistry,
		StopOnError: DefaultStopOnError,
	}
}

// Options holds the options for validation.
type Options struct {
	Registry         *Registry
	StopOnError      bool
	WarningsAsErrors bool
	Validator        Validator
}

func (o *Options) shouldStopOnWarnings() bool {
	return o.StopOnError && o.WarningsAsErrors
}

// Option provides the ability to alter options.
type Option func(*Options)

// WithRegistry makes an option for using a specific Registry.
func WithRegistry(r *Registry) Option {
	return func(opts *Options) {
		opts.Registry = r
	}
}

// WithStopOnError indicates to the validator to stop when it finds an error.
func WithStopOnError() Option {
	return func(opts *Options) {
		opts.StopOnError = true
	}
}

// WithWarningsAsErrors indicates that warnings should be treated as errors.
func WithWarningsAsErrors() Option {
	return func(opts *Options) {
		opts.WarningsAsErrors = true
	}
}

// WithValidator indicates the validator to use, skipping the registry.
func WithValidator(validator Validator) Option {
	return func(opts *Options) {
		opts.Validator = validator
	}
}
