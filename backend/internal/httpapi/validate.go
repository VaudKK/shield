package httpapi

import (
	"net/mail"
	"strings"
)

const (
	minPasswordLength = 10
	maxPasswordLength = 128
	maxDisplayNameLen = 100
)

// fieldErrors collects validation problems so a handler can report all of
// them at once instead of failing on the first field.
type fieldErrors []string

func (f fieldErrors) messages() string {
	return strings.Join(f, " ")
}

func validateEmail(email string) fieldErrors {
	var errs fieldErrors
	if _, err := mail.ParseAddress(email); err != nil {
		errs = append(errs, "Enter a valid email address.")
	}
	return errs
}

func validatePassword(password string) fieldErrors {
	var errs fieldErrors
	if len(password) < minPasswordLength {
		errs = append(errs, "Password must be at least 10 characters long.")
	}
	if len(password) > maxPasswordLength {
		errs = append(errs, "Password is too long.")
	}
	return errs
}

func validateDisplayName(name string) fieldErrors {
	var errs fieldErrors
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		errs = append(errs, "Display name is required.")
	}
	if len(trimmed) > maxDisplayNameLen {
		errs = append(errs, "Display name is too long.")
	}
	return errs
}
