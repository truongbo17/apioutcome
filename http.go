package apioutcome

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/felixge/httpsnoop"
)

// Handler is an HTTP handler that can return an error before writing a
// response. A returned error after writing cannot change the HTTP response.
type Handler func(http.ResponseWriter, *http.Request) error

// Options configures the completion logger. A nil Logger uses slog.Default.
type Options struct{ Logger *slog.Logger }

// Middleware observes an existing http.Handler without changing its signature.
// Install it once around the router so unmatched routes are observed too.
func Middleware(opts Options) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, nested := r.Context().Value(stateKey{}).(*responseState); nested {
				next.ServeHTTP(w, r)
				return
			}
			started := time.Now()
			state := &responseState{status: http.StatusOK}
			r = r.WithContext(context.WithValue(r.Context(), stateKey{}, state))
			tracked := state.wrap(w)
			defer func() {
				if recover() != nil {
					WriteError(tracked, r, errors.New("handler panic"))
				}
				if state.code == "" {
					state.code = "ok"
					if state.status >= 400 {
						state.code = "unclassified_http_error"
					}
				}
				Report(r.Context(), opts.Logger, Event{
					Method: r.Method, Status: state.status, Code: state.code,
					ErrorAfterCommit: state.errorAfterCommit,
					Duration:         time.Since(started),
				})
			}()
			next.ServeHTTP(tracked, r)
		})
	}
}

// Wrap adapts a returned-error handler. Existing handlers can use Middleware
// and WriteError instead, without changing their signature.
func Wrap(handler Handler, opts Options) http.Handler {
	return Middleware(opts)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := handler(w, r); err != nil {
			WriteError(w, r, err)
		}
	}))
}

// NotFound returns an HTTP fallback handler for unmatched routes. Register it
// on "/" after more specific routes when using http.ServeMux.
func NotFound(opts Options) http.Handler {
	return Wrap(func(http.ResponseWriter, *http.Request) error {
		return Problem(http.StatusNotFound, "not_found", "", nil)
	}, opts)
}

// WriteError records a public code and writes a problem response. When called
// under Middleware after the response is committed, it only records the code.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	problem := classify(err)
	if r != nil {
		if state, ok := r.Context().Value(stateKey{}).(*responseState); ok {
			state.code = problem.Code
			if state.committed {
				state.errorAfterCommit = true
				return
			}
		}
	}
	Render(w, r, err)
}

// Render writes an RFC 9457-shaped problem response and returns its public
// description. The caller must ensure the response has not been committed.
func Render(w http.ResponseWriter, r *http.Request, err error) *Error {
	problem := classify(err)
	w.Header().Set("Content-Type", "application/problem+json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(problem.Status)
	if r == nil || r.Method != http.MethodHead {
		_ = json.NewEncoder(w).Encode(struct {
			Type   string `json:"type"`
			Title  string `json:"title"`
			Status int    `json:"status"`
			Code   string `json:"code"`
			Detail string `json:"detail,omitempty"`
		}{Type: "about:blank", Title: http.StatusText(problem.Status), Status: problem.Status, Code: problem.Code, Detail: problem.Detail})
	}
	return problem
}

type responseState struct {
	status           int
	committed        bool
	code             string
	errorAfterCommit bool
}

type stateKey struct{}

func (s *responseState) commit(status int) {
	if !s.committed {
		s.committed = true
		s.status = status
	}
}

func (s *responseState) wrap(w http.ResponseWriter) http.ResponseWriter {
	return httpsnoop.Wrap(w, httpsnoop.Hooks{
		WriteHeader: func(next httpsnoop.WriteHeaderFunc) httpsnoop.WriteHeaderFunc {
			return func(status int) {
				if status >= 200 || status == http.StatusSwitchingProtocols {
					s.commit(status)
				}
				next(status)
			}
		},
		Write: func(next httpsnoop.WriteFunc) httpsnoop.WriteFunc {
			return func(b []byte) (int, error) { s.commit(http.StatusOK); return next(b) }
		},
		ReadFrom: func(next httpsnoop.ReadFromFunc) httpsnoop.ReadFromFunc {
			return func(src io.Reader) (int64, error) { s.commit(http.StatusOK); return next(src) }
		},
		Flush: func(next httpsnoop.FlushFunc) httpsnoop.FlushFunc {
			return func() { s.commit(http.StatusOK); next() }
		},
		Hijack: func(next httpsnoop.HijackFunc) httpsnoop.HijackFunc {
			return func() (net.Conn, *bufio.ReadWriter, error) {
				conn, rw, err := next()
				if err == nil {
					s.commit(http.StatusSwitchingProtocols)
				}
				return conn, rw, err
			}
		},
	})
}
