package exceptions

import "fmt"

type MonalisaError struct {
	Message string
}

func (e *MonalisaError) Error() string {
	return e.Message
}

func NewLicenseError(msg string, args ...interface{}) error {
	return &MonalisaError{Message: fmt.Sprintf(msg, args...)}
}

func NewModuleError(msg string, args ...interface{}) error {
	return &MonalisaError{Message: fmt.Sprintf(msg, args...)}
}

func NewSessionError(msg string, args ...interface{}) error {
	return &MonalisaError{Message: fmt.Sprintf(msg, args...)}
}
