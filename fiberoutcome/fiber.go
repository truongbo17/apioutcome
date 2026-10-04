// Package fiberoutcome adapts apioutcome to Fiber v2.
package fiberoutcome

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
	outcome "github.com/truongbo17/apioutcome"
)

// Middleware observes existing Fiber handlers and converts returned errors to
// safe problem responses. Register it before routes. Fiber's default error
// handler is not called for errors handled by this middleware.
func Middleware(opts outcome.Options) fiber.Handler {
	return func(c *fiber.Ctx) (returnErr error) {
		started := time.Now()
		code := "ok"
		afterCommit := false
		defer func() {
			if recover() != nil {
				returnErr = errors.New("handler panic")
			}
			if returnErr != nil {
				problem := classify(returnErr)
				code = problem.Code
				if len(c.Response().Body()) > 0 || c.Response().StatusCode() != http.StatusOK {
					afterCommit = true
				} else {
					writeProblem(c, problem)
				}
				returnErr = nil
			} else if c.Response().StatusCode() >= 400 {
				code = "unclassified_http_error"
			}
			outcome.Report(c.UserContext(), opts.Logger, outcome.Event{
				Method: c.Method(), Status: c.Response().StatusCode(), Code: code,
				ErrorAfterCommit: afterCommit, Duration: time.Since(started),
			})
		}()
		return c.Next()
	}
}

func classify(err error) *outcome.Error {
	var public *outcome.Error
	if errors.As(err, &public) {
		return outcome.AsProblem(err)
	}
	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) && fiberErr.Code >= 400 && fiberErr.Code <= 599 {
		code := "http_error"
		if fiberErr.Code == http.StatusNotFound {
			code = "not_found"
		}
		return outcome.Problem(fiberErr.Code, code, "", err)
	}
	return outcome.AsProblem(err)
}

func writeProblem(c *fiber.Ctx, problem *outcome.Error) {
	data, _ := json.Marshal(struct {
		Type   string `json:"type"`
		Title  string `json:"title"`
		Status int    `json:"status"`
		Code   string `json:"code"`
		Detail string `json:"detail,omitempty"`
	}{"about:blank", http.StatusText(problem.Status), problem.Status, problem.Code, problem.Detail})
	c.Status(problem.Status)
	c.Set("Content-Type", "application/problem+json")
	c.Set("Cache-Control", "no-store")
	c.Set("X-Content-Type-Options", "nosniff")
	if c.Method() != http.MethodHead {
		_ = c.Send(data)
	}
}
