// Package domain holds Shield's core data types, shared across service and
// repository layers.
package domain

import "errors"

var ErrNotFound = errors.New("not found")
