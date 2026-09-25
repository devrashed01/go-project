// Package validator collects field-level validation errors.
package validator

// Error is returned when input fails validation. Fields maps a field name to
// a human-readable problem, e.g. {"title": "must not be empty"}.
type Error struct {
	Fields map[string]string
}

func (e *Error) Error() string { return "validation failed" }

// Validator accumulates validation failures.
type Validator struct {
	fields map[string]string
}

// Check records message for field when ok is false. Only the first failure
// per field is kept.
func (v *Validator) Check(ok bool, field, message string) {
	if ok {
		return
	}
	if v.fields == nil {
		v.fields = make(map[string]string)
	}
	if _, exists := v.fields[field]; !exists {
		v.fields[field] = message
	}
}

// Err returns an *Error if any check failed, or nil otherwise.
func (v *Validator) Err() error {
	if len(v.fields) == 0 {
		return nil
	}
	return &Error{Fields: v.fields}
}
