package fiberv3outcome_test

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	outcome "github.com/truongbo17/apioutcome"
	"github.com/truongbo17/apioutcome/fiberv3outcome"
)

func TestFiberMiddleware(t *testing.T) {
	var log bytes.Buffer
	app := fiber.New()
	app.Use(fiberv3outcome.Middleware(outcome.Options{Logger: slog.New(slog.NewJSONHandler(&log, nil))}))
	app.Get("/ok", func(c fiber.Ctx) error { return c.SendStatus(http.StatusNoContent) })
	app.Post("/orders", func(fiber.Ctx) error {
		return outcome.Problem(422, "invalid_order", "Missing product", errors.New("secret"))
	})
	app.Get("/wrapped", func(fiber.Ctx) error {
		return outcome.Problem(422, "invalid_order", "Missing product", fiber.NewError(400, "secret"))
	})
	for _, tc := range []struct {
		method, path string
		status       int
		code         string
	}{
		{http.MethodGet, "/ok", 204, "ok"},
		{http.MethodPost, "/orders", 422, "invalid_order"},
		{http.MethodGet, "/wrapped", 422, "invalid_order"},
		{http.MethodGet, "/missing", 404, "not_found"},
	} {
		response, err := app.Test(httprequest(tc.method, tc.path))
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(response.Body)
		_ = response.Body.Close()
		if response.StatusCode != tc.status {
			t.Fatalf("%s status = %d", tc.path, response.StatusCode)
		}
		if tc.code != "ok" && !strings.Contains(string(body), `"code":"`+tc.code+`"`) {
			t.Fatalf("%s body = %q", tc.path, body)
		}
		if !strings.Contains(log.String(), `"code":"`+tc.code+`"`) {
			t.Fatalf("%s log = %q", tc.path, log.String())
		}
	}
	if strings.Contains(log.String(), "secret") || strings.Count(log.String(), "\n") != 4 {
		t.Fatalf("unsafe or duplicate logs: %q", log.String())
	}
}

func TestFiberPreservesBodyAfterError(t *testing.T) {
	var log bytes.Buffer
	app := fiber.New()
	app.Use(fiberv3outcome.Middleware(outcome.Options{Logger: slog.New(slog.NewJSONHandler(&log, nil))}))
	app.Get("/partial", func(c fiber.Ctx) error {
		if err := c.SendString("already sent"); err != nil {
			return err
		}
		return errors.New("private failure")
	})
	response, err := app.Test(httprequest(http.MethodGet, "/partial"))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if string(body) != "already sent" || !strings.Contains(log.String(), `"error_after_commit":true`) {
		t.Fatalf("response/log = %q %q", body, log.String())
	}
}

func httprequest(method, path string) *http.Request {
	request, _ := http.NewRequest(method, "http://example.test"+path, nil)
	return request
}
