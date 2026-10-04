package apioutcome_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gofiber/fiber/v2"
	outcome "github.com/truongbo17/apioutcome"
	"github.com/truongbo17/apioutcome/fiberoutcome"
	"github.com/truongbo17/apioutcome/ginoutcome"
)

// Benchmarks measure in-process request handling. No socket, exporter, or
// network I/O is included. Baselines use the same framework and recorder.
func BenchmarkHTTP(b *testing.B) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	opts := outcome.Options{Logger: logger}
	plain := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	failure := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		outcome.WriteError(w, r, outcome.Problem(422, "invalid_order", "Invalid order", nil))
	})
	cases := []struct {
		name string
		h    http.Handler
	}{
		{"plain_204", plain},
		{"middleware_204", outcome.Middleware(opts)(plain)},
		{"wrap_204", outcome.Wrap(func(w http.ResponseWriter, r *http.Request) error { plain.ServeHTTP(w, r); return nil }, opts)},
		{"plain_422", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(422) })},
		{"middleware_422", outcome.Middleware(opts)(failure)},
		{"wrap_422", outcome.Wrap(func(http.ResponseWriter, *http.Request) error {
			return outcome.Problem(422, "invalid_order", "Invalid order", nil)
		}, opts)},
	}
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				tc.h.ServeHTTP(httptest.NewRecorder(), request)
			}
		})
	}
}

func BenchmarkGin(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	opts := outcome.Options{Logger: slog.New(slog.NewJSONHandler(io.Discard, nil))}
	for _, withMiddleware := range []bool{false, true} {
		app := gin.New()
		if withMiddleware {
			app.Use(ginoutcome.Middleware(opts))
		}
		app.GET("/", func(c *gin.Context) { c.Status(204) })
		name := "plain_204"
		if withMiddleware {
			name = "middleware_204"
		}
		b.Run(name, func(b *testing.B) {
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				app.ServeHTTP(httptest.NewRecorder(), request)
			}
		})
	}
	app := gin.New()
	app.Use(ginoutcome.Middleware(opts))
	app.GET("/", func(c *gin.Context) {
		ginoutcome.WriteError(c, outcome.Problem(422, "invalid_order", "Invalid order", nil))
	})
	b.Run("middleware_422", func(b *testing.B) {
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			app.ServeHTTP(httptest.NewRecorder(), request)
		}
	})
}

func BenchmarkFiberV2(b *testing.B) {
	opts := outcome.Options{Logger: slog.New(slog.NewJSONHandler(io.Discard, nil))}
	for _, withMiddleware := range []bool{false, true} {
		app := fiber.New()
		if withMiddleware {
			app.Use(fiberoutcome.Middleware(opts))
		}
		app.Get("/", func(c *fiber.Ctx) error { return c.SendStatus(204) })
		name := "plain_204"
		if withMiddleware {
			name = "middleware_204"
		}
		b.Run(name, func(b *testing.B) {
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				response, err := app.Test(request, -1)
				if err != nil {
					b.Fatal(err)
				}
				_ = response.Body.Close()
			}
		})
	}
	app := fiber.New()
	app.Use(fiberoutcome.Middleware(opts))
	app.Get("/", func(*fiber.Ctx) error { return outcome.Problem(422, "invalid_order", "Invalid order", nil) })
	b.Run("middleware_422", func(b *testing.B) {
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			response, err := app.Test(request, -1)
			if err != nil {
				b.Fatal(err)
			}
			_ = response.Body.Close()
		}
	})
}
