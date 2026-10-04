package fiberv3outcome_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	outcome "github.com/truongbo17/apioutcome"
	"github.com/truongbo17/apioutcome/fiberv3outcome"
)

func BenchmarkFiberV3(b *testing.B) {
	opts := outcome.Options{Logger: slog.New(slog.NewJSONHandler(io.Discard, nil))}
	for _, withMiddleware := range []bool{false, true} {
		app := fiber.New()
		if withMiddleware {
			app.Use(fiberv3outcome.Middleware(opts))
		}
		app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(204) })
		name := "plain_204"
		if withMiddleware {
			name = "middleware_204"
		}
		b.Run(name, func(b *testing.B) {
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				response, err := app.Test(request, fiber.TestConfig{Timeout: 0})
				if err != nil {
					b.Fatal(err)
				}
				_ = response.Body.Close()
			}
		})
	}
	app := fiber.New()
	app.Use(fiberv3outcome.Middleware(opts))
	app.Get("/", func(fiber.Ctx) error { return outcome.Problem(422, "invalid_order", "Invalid order", nil) })
	b.Run("middleware_422", func(b *testing.B) {
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			response, err := app.Test(request, fiber.TestConfig{Timeout: 0})
			if err != nil {
				b.Fatal(err)
			}
			_ = response.Body.Close()
		}
	})
}
