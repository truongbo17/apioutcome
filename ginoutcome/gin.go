// Package ginoutcome adapts apioutcome to Gin middleware and handlers.
package ginoutcome

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	outcome "github.com/truongbo17/apioutcome"
)

const stateKey = "apioutcome/gin/state"

type state struct {
	code             string
	errorAfterCommit bool
}

// Handler is a Gin handler that returns an error to apioutcome.
type Handler func(*gin.Context) error

// Middleware observes existing Gin handlers without changing their signature.
// Register it before routes. Use WriteError for a stable code and response.
func Middleware(opts outcome.Options) gin.HandlerFunc {
	return func(c *gin.Context) {
		observe(c, func(c *gin.Context) error { c.Next(); return nil }, opts)
	}
}

// Wrap adapts a returned-error handler. It cooperates with Middleware so the
// same request is reported once when both are installed.
func Wrap(handler Handler, opts outcome.Options) gin.HandlerFunc {
	return func(c *gin.Context) { observe(c, handler, opts) }
}

func observe(c *gin.Context, handler Handler, opts outcome.Options) {
	if _, nested := getState(c); nested {
		if err := handler(c); err != nil {
			WriteError(c, err)
		}
		return
	}
	started := time.Now()
	s := &state{}
	c.Set(stateKey, s)
	defer func() {
		if recover() != nil {
			WriteError(c, errors.New("handler panic"))
		}
		if s.code == "" {
			s.code = "ok"
			if c.Writer.Status() >= 400 {
				s.code = "unclassified_http_error"
			}
		}
		outcome.Report(c.Request.Context(), opts.Logger, outcome.Event{
			Method: c.Request.Method, Status: c.Writer.Status(), Code: s.code,
			ErrorAfterCommit: s.errorAfterCommit,
			Duration:         time.Since(started),
		})
	}()
	if err := handler(c); err != nil {
		WriteError(c, err)
	}
}

func getState(c *gin.Context) (*state, bool) {
	value, exists := c.Get(stateKey)
	s, ok := value.(*state)
	return s, exists && ok
}

// WriteError renders a problem response and records its safe code. If Gin has
// already written a response, it preserves that response and records the error.
func WriteError(c *gin.Context, err error) {
	problem := outcome.AsProblem(err)
	if s, ok := getState(c); ok {
		s.code = problem.Code
		if c.Writer.Written() {
			s.errorAfterCommit = true
			c.Abort()
			return
		}
	}
	outcome.Render(c.Writer, c.Request, err)
	c.Abort()
}

// NotFound returns a Gin NoRoute handler producing the shared problem format.
func NotFound(opts outcome.Options) gin.HandlerFunc {
	return Wrap(func(*gin.Context) error {
		return outcome.Problem(http.StatusNotFound, "not_found", "", nil)
	}, opts)
}
