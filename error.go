// Package apioutcome connects returned HTTP handler errors to safe client
// responses, structured completion logs, and the active OpenTelemetry span.
package apioutcome

import (
	"errors"
	"net/http"
)

// Error describes a public API failure. Cause remains private: apioutcome
// never puts it in an HTTP response, completion log, or trace attribute.
type Error struct {
	Status int
	Code   string
	Detail string
	Cause  error
}

func (e *Error) Error() string { return e.Code }

// Unwrap makes Cause available to application code using errors.Is/As.
func (e *Error) Unwrap() error { return e.Cause }

// Problem constructs a public error. Code and detail must be constants chosen
// by the application, never strings copied from client input or error causes.
func Problem(status int, code, detail string, cause error) *Error {
	if status < 400 || status > 599 || !validCode(code) {
		return &Error{Status: http.StatusInternalServerError, Code: "internal_error", Cause: cause}
	}
	if status >= 500 {
		detail = ""
	}
	return &Error{Status: status, Code: code, Detail: detail, Cause: cause}
}

func validCode(code string) bool {
	if code == "" {
		return false
	}
	for _, c := range code {
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '_' {
			return false
		}
	}
	return true
}

func classify(err error) *Error {
	var apiErr *Error
	if errors.As(err, &apiErr) && apiErr != nil && apiErr.Status >= 400 && apiErr.Status <= 599 && validCode(apiErr.Code) {
		return Problem(apiErr.Status, apiErr.Code, apiErr.Detail, apiErr.Cause)
	}
	return Problem(500, "internal_error", "", err)
}

// AsProblem returns the public classification of err. Unknown errors become
// an internal_error with no private details in its public fields.
func AsProblem(err error) *Error { return classify(err) }
