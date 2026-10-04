// Package ginoutcome adapts apioutcome to Gin handlers that return errors.
package ginoutcome

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	outcome "github.com/truongbo17/apioutcome"
)

// Handler is a Gin handler that returns an error to apioutcome.
type Handler func(*gin.Context) error

// Wrap renders returned errors before Gin commits the response, then records
// one completion event. DecodeJSON can be used inside the wrapped handler.
func Wrap(handler Handler, opts outcome.Options) gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		var handlerErr error
		defer func() {
			if recover() != nil {
				handlerErr = errors.New("handler panic")
			}
			code := "ok"
			committedFailure := false
			if handlerErr != nil {
				problem := outcome.AsProblem(handlerErr)
				code = problem.Code
				committedFailure = c.Writer.Written()
				if !committedFailure {
					outcome.Render(c.Writer, c.Request, handlerErr)
				}
				c.Abort()
			} else if c.Writer.Status() >= 400 {
				code = "unclassified_http_error"
			}
			outcome.Report(c.Request.Context(), opts.Logger, outcome.Event{
				Method: c.Request.Method, Status: c.Writer.Status(), Code: code,
				ErrorAfterCommit: committedFailure,
				Duration:         time.Since(started),
			})
		}()
		handlerErr = handler(c)
	}
}

// NotFound returns a Gin NoRoute handler producing the same problem format.
func NotFound(opts outcome.Options) gin.HandlerFunc {
	return Wrap(func(*gin.Context) error {
		return outcome.Problem(http.StatusNotFound, "not_found", "Not Found", nil)
	}, opts)
}
