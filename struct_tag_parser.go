package validate

import (
	"fmt"
	"strings"
	"unicode"
)

// DefaultStructTagName is the default tag name to use when building validators from struct tags.
var DefaultStructTagName = "validate"

// StructTagParser is responsible for parsing struct tags and producing a validator.
type StructTagParser interface {
	ParseStructTags(ResolutionContext, string) (*StructTagParseResult, error)
}

// StructTagParserFunc is an adapter for a function that implements the StructTagParser interface.
type StructTagParserFunc func(ResolutionContext, string) (*StructTagParseResult, error)

// ParseStructTags implements the StructTagParser interface.
func (f StructTagParserFunc) ParseStructTags(ctx ResolutionContext, tagName string) (*StructTagParseResult, error) {
	return f(ctx, tagName)
}

// StructTagParseResult is returned after parsing a struct tag.
type StructTagParseResult struct {
	Validator     Validator
	CustomMessage string
}

type pState uint8

const (
	none pState = iota
	name
	args
)

// DefaultStructTagParser is the default StructTagParser.
var DefaultStructTagParser StructTagParserFunc = func(ctx ResolutionContext, tagName string) (*StructTagParseResult, error) {
	tag, ok := ctx.StructField.Tag.Lookup(tagName)
	if !ok || tag == "-" {
		return &StructTagParseResult{Validator: NoOpValidator{}}, nil
	}

	defAndMessage := strings.SplitN(tag, "~", 2)

	parts := strings.Split(defAndMessage[0], `|`)

	var validators []Validator
	for _, disjunctionPart := range parts {
		v, err := parseTag(ctx, []rune(disjunctionPart))
		if err != nil {
			return nil, err
		}

		validators = append(validators, v)
	}

	stpr := &StructTagParseResult{Validator: Or(validators...)}

	if len(defAndMessage) == 2 {
		stpr.CustomMessage = defAndMessage[1]
	}

	return stpr, nil
}

func parseTag(ctx ResolutionContext, tag []rune) (Validator, error) {
	state := none

	var validators []Validator

	var validatorName string
	var validatorArg string
	var validatorArgs []string

	for i := 0; i < len(tag)+1; i++ {
		var c rune
		if i == len(tag) {
			c = 0
		} else {
			c = tag[i]
		}

		switch state {
		case none:
			switch {
			case c == '_' || unicode.IsLetter(c) || unicode.IsDigit(c):
				state = name
				validatorName += string(c)
			case unicode.IsSpace(c):
			case c == '(':
				state = args
			case c == ',' || c == 0:
				vf, err := ctx.LookupTagValidatorFactory(validatorName)
				if err != nil {
					return nil, err
				}

				v, err := vf.Create(ctx, validatorName, validatorArgs)
				if err != nil {
					return nil, err
				}

				validators = append(validators, v)
				state = none
				validatorName = ""
			default:
				return nil, fmt.Errorf("invalid character %q", c)
			}
		case name:
			switch {
			case c == '_' || unicode.IsLetter(c) || unicode.IsDigit(c):
				validatorName += string(c)
			default:
				state = none
				i--
			}
		case args:
			switch c {
			case ',':
				validatorArgs = append(validatorArgs, validatorArg)
				validatorArg = ""
			case ')':
				validatorArgs = append(validatorArgs, validatorArg)
				validatorArg = ""
				state = none
			default:
				validatorArg += string(c)
			}
		}
	}

	return And(validators...), nil
}
