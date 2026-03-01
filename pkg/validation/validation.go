// Package validation provides simple input validation utilities.
// It uses struct tags for declarative validation rules.
package validation

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

// Tag name for validation rules
const tagName = "validate"

// Rules supported:
//   - required: field must not be empty
//   - min=x: minimum value for numbers or length for strings
//   - max=x: maximum value for numbers or length for strings
//   - email: must be a valid email
//   - url: must be a valid URL
//   - oneof=a|b|c: must be one of the specified values
//   - uuid: must be a valid UUID

// Validator holds validation errors
type Validator struct {
	Errors map[string]string
}

// New creates a new Validator
func New() *Validator {
	return &Validator{
		Errors: make(map[string]string),
	}
}

// Valid returns true if no validation errors
func (v *Validator) Valid() bool {
	return len(v.Errors) == 0
}

// AddError adds a validation error
func (v *Validator) AddError(field, message string) {
	if v.Errors == nil {
		v.Errors = make(map[string]string)
	}
	v.Errors[field] = message
}

// Validate validates a struct using validation tags
func (v *Validator) Validate(s interface{}) bool {
	val := reflect.ValueOf(s)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return true
	}

	typ := val.Type()
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		fieldValue := val.Field(i)

		// Get validation tag
		tag := field.Tag.Get(tagName)
		if tag == "" || tag == "-" {
			continue
		}

		// Parse and apply rules
		rules := strings.Split(tag, ",")
		for _, rule := range rules {
			if err := v.applyRule(field.Name, fieldValue, rule); err != nil {
				v.AddError(field.Name, err.Error())
				break // Stop at first error for this field
			}
		}

		// Recursively validate nested structs
		if fieldValue.Kind() == reflect.Struct && fieldValue.CanInterface() {
			v.Validate(fieldValue.Interface())
		} else if fieldValue.Kind() == reflect.Ptr && !fieldValue.IsNil() && fieldValue.Elem().Kind() == reflect.Struct {
			v.Validate(fieldValue.Interface())
		}
	}

	return v.Valid()
}

// applyRule applies a single validation rule
func (v *Validator) applyRule(name string, value reflect.Value, rule string) error {
	parts := strings.SplitN(rule, "=", 2)
	ruleName := parts[0]
	ruleParam := ""
	if len(parts) > 1 {
		ruleParam = parts[1]
	}

	switch ruleName {
	case "required":
		return validateRequired(value)
	case "min":
		return validateMin(value, ruleParam)
	case "max":
		return validateMax(value, ruleParam)
	case "email":
		return validateEmail(value)
	case "url":
		return validateURL(value)
	case "oneof":
		return validateOneOf(value, ruleParam)
	case "uuid":
		return validateUUID(value)
	default:
		return nil // Unknown rules are ignored
	}
}

func validateRequired(value reflect.Value) error {
	if !value.IsValid() {
		return fmt.Errorf("is required")
	}

	switch value.Kind() {
	case reflect.String:
		if strings.TrimSpace(value.String()) == "" {
			return fmt.Errorf("is required")
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if value.Int() == 0 {
			return fmt.Errorf("is required")
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if value.Uint() == 0 {
			return fmt.Errorf("is required")
		}
	case reflect.Ptr, reflect.Slice, reflect.Map, reflect.Interface:
		if value.IsNil() {
			return fmt.Errorf("is required")
		}
	}
	return nil
}

func validateMin(value reflect.Value, param string) error {
	if param == "" {
		return nil
	}

	var minVal int
	fmt.Sscanf(param, "%d", &minVal)

	switch value.Kind() {
	case reflect.String:
		if len(value.String()) < minVal {
			return fmt.Errorf("must be at least %d characters", minVal)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if value.Int() < int64(minVal) {
			return fmt.Errorf("must be at least %d", minVal)
		}
	case reflect.Slice, reflect.Map:
		if value.Len() < minVal {
			return fmt.Errorf("must have at least %d items", minVal)
		}
	}
	return nil
}

func validateMax(value reflect.Value, param string) error {
	if param == "" {
		return nil
	}

	var maxVal int
	fmt.Sscanf(param, "%d", &maxVal)

	switch value.Kind() {
	case reflect.String:
		if len(value.String()) > maxVal {
			return fmt.Errorf("must be at most %d characters", maxVal)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if value.Int() > int64(maxVal) {
			return fmt.Errorf("must be at most %d", maxVal)
		}
	case reflect.Slice, reflect.Map:
		if value.Len() > maxVal {
			return fmt.Errorf("must have at most %d items", maxVal)
		}
	}
	return nil
}

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

func validateEmail(value reflect.Value) error {
	if value.Kind() != reflect.String {
		return nil
	}
	if value.String() == "" {
		return nil // Use "required" for empty check
	}
	if !emailRegex.MatchString(value.String()) {
		return fmt.Errorf("must be a valid email address")
	}
	return nil
}

var urlRegex = regexp.MustCompile(`^https?://[^\s/$.?#].[^\s]*$`)

func validateURL(value reflect.Value) error {
	if value.Kind() != reflect.String {
		return nil
	}
	if value.String() == "" {
		return nil // Use "required" for empty check
	}
	if !urlRegex.MatchString(value.String()) {
		return fmt.Errorf("must be a valid URL")
	}
	return nil
}

func validateOneOf(value reflect.Value, param string) error {
	if param == "" {
		return nil
	}
	allowed := strings.Split(param, "|")
	strVal := fmt.Sprintf("%v", value.Interface())
	for _, a := range allowed {
		if strVal == a {
			return nil
		}
	}
	return fmt.Errorf("must be one of: %s", param)
}

var uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func validateUUID(value reflect.Value) error {
	if value.Kind() != reflect.String {
		return nil
	}
	if value.String() == "" {
		return nil // Use "required" for empty check
	}
	if !uuidRegex.MatchString(value.String()) {
		return fmt.Errorf("must be a valid UUID")
	}
	return nil
}

// ValidateStruct is a convenience function that validates a struct and returns errors
func ValidateStruct(s interface{}) map[string]string {
	v := New()
	v.Validate(s)
	return v.Errors
}
