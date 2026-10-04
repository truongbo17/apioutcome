package apioutcome_test

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	outcome "github.com/truongbo17/apioutcome"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestReturnedErrorsShareResponseAndSafeLog(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		code   string
		title  string
	}{
		{"public", outcome.Problem(422, "invalid_order", "Invalid order", errors.New("card 4242 secret")), 422, "invalid_order", http.StatusText(422)},
		{"private", errors.New("password=hunter2"), 500, "internal_error", "Internal Server Error"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var log bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&log, nil))
			handler := outcome.Wrap(func(http.ResponseWriter, *http.Request) error { return tc.err }, outcome.Options{Logger: logger})
			req := httptest.NewRequest(http.MethodPost, "/orders?token=secret", nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, req)

			if response.Code != tc.status {
				t.Fatalf("status = %d, want %d", response.Code, tc.status)
			}
			if got := response.Header().Get("Content-Type"); got != "application/problem+json" {
				t.Fatalf("Content-Type = %q", got)
			}
			body := response.Body.String()
			for _, want := range []string{`"type":"about:blank"`, `"code":"` + tc.code + `"`, `"title":"` + tc.title + `"`} {
				if !strings.Contains(body, want) {
					t.Errorf("body %q missing %q", body, want)
				}
			}
			if strings.Contains(body, "secret") || strings.Contains(body, "hunter2") || strings.Contains(log.String(), "secret") || strings.Contains(log.String(), "hunter2") {
				t.Fatalf("sensitive data leaked: body=%q log=%q", body, log.String())
			}
			if strings.Count(strings.TrimSpace(log.String()), "\n") != 0 {
				t.Fatalf("expected one log record, got %q", log.String())
			}
			if !strings.Contains(log.String(), `"code":"`+tc.code+`"`) {
				t.Fatalf("log missing code: %q", log.String())
			}
		})
	}
}

func TestCommittedResponseIsPreserved(t *testing.T) {
	var log bytes.Buffer
	handler := outcome.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		w.WriteHeader(http.StatusAccepted)
		_, _ = io.WriteString(w, "already sent")
		return errors.New("private detail")
	}, outcome.Options{Logger: slog.New(slog.NewJSONHandler(&log, nil))})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusAccepted || response.Body.String() != "already sent" {
		t.Fatalf("response changed: %#v", response)
	}
	if !strings.Contains(log.String(), `"error_after_commit":true`) {
		t.Fatalf("commit state missing: %q", log.String())
	}
}

func TestDecodeJSON(t *testing.T) {
	tests := []struct {
		name, body string
		limit      int64
		wantCode   string
	}{
		{"valid", `{"name":"Ada"}`, 100, ""},
		{"malformed", `{"name":`, 100, "invalid_json"},
		{"extra", `{} {}`, 100, "invalid_json"},
		{"unknown field", `{"other":1}`, 100, "invalid_json"},
		{"oversized", `{"name":"Ada"}`, 5, "body_too_large"},
		{"max limit", `{"name":"Ada"}`, math.MaxInt64, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tc.body))
			var input struct {
				Name string `json:"name"`
			}
			err := outcome.DecodeJSON(req, &input, tc.limit)
			if tc.wantCode == "" {
				if err != nil || input.Name != "Ada" {
					t.Fatalf("result = %+v, %v", input, err)
				}
				return
			}
			var problem *outcome.Error
			if !errors.As(err, &problem) || problem.Code != tc.wantCode {
				t.Fatalf("error = %v, want %q", err, tc.wantCode)
			}
		})
	}
}

func TestPanicProducesSafeFailure(t *testing.T) {
	var log bytes.Buffer
	handler := outcome.Wrap(func(http.ResponseWriter, *http.Request) error {
		panic("private panic value")
	}, outcome.Options{Logger: slog.New(slog.NewJSONHandler(&log, nil))})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != 500 || !strings.Contains(response.Body.String(), `"code":"internal_error"`) {
		t.Fatalf("unsafe panic response: %d %q", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String()+log.String(), "private panic value") {
		t.Fatal("panic content leaked")
	}
}

func TestCommittedHeaderWithoutBodyIsPreserved(t *testing.T) {
	handler := outcome.Wrap(func(w http.ResponseWriter, _ *http.Request) error {
		w.WriteHeader(http.StatusNoContent)
		return errors.New("later failure")
	}, outcome.Options{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusNoContent || response.Body.Len() != 0 {
		t.Fatalf("committed header changed: %d %q", response.Code, response.Body.String())
	}
}

func TestHEADProblemHasNoBody(t *testing.T) {
	handler := outcome.Wrap(func(http.ResponseWriter, *http.Request) error { return errors.New("secret") }, outcome.Options{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodHead, "/", nil))
	if response.Code != 500 || response.Body.Len() != 0 {
		t.Fatalf("HEAD response = %d %q", response.Code, response.Body.String())
	}
}

func TestExistingSpanReceivesSafeOutcome(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	defer func() { _ = provider.Shutdown(t.Context()) }()
	ctx, span := provider.Tracer("apioutcome-test").Start(t.Context(), "request")

	var log bytes.Buffer
	handler := outcome.Wrap(func(http.ResponseWriter, *http.Request) error {
		return errors.New("token=top-secret")
	}, outcome.Options{Logger: slog.New(slog.NewJSONHandler(&log, nil))})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/", nil).WithContext(ctx))
	span.End()

	spans := recorder.Ended()
	if len(spans) != 1 || spans[0].Status().Code != codes.Error {
		t.Fatalf("span status = %+v", spans)
	}
	foundCode := false
	for _, attr := range spans[0].Attributes() {
		if string(attr.Key) == "apioutcome.code" && attr.Value.AsString() == "internal_error" {
			foundCode = true
		}
	}
	if !foundCode {
		t.Fatalf("span has no safe error code: %+v", spans[0].Attributes())
	}
	if !strings.Contains(log.String(), span.SpanContext().TraceID().String()) {
		t.Fatalf("log missing trace ID: %q", log.String())
	}
	if strings.Contains(log.String(), "top-secret") {
		t.Fatalf("secret leaked in log: %q", log.String())
	}
}

func TestServerProblemCannotExposeDetail(t *testing.T) {
	handler := outcome.Wrap(func(http.ResponseWriter, *http.Request) error {
		return outcome.Problem(503, "storage_error", "password=secret", nil)
	}, outcome.Options{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != 503 || strings.Contains(response.Body.String(), "secret") {
		t.Fatalf("server detail leaked: %d %q", response.Code, response.Body.String())
	}
}

func TestUnclassifiedHTTPErrorIsVisible(t *testing.T) {
	var log bytes.Buffer
	handler := outcome.Wrap(func(w http.ResponseWriter, _ *http.Request) error {
		w.WriteHeader(403)
		return nil
	}, outcome.Options{Logger: slog.New(slog.NewJSONHandler(&log, nil))})
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if !strings.Contains(log.String(), `"code":"unclassified_http_error"`) {
		t.Fatalf("missing unclassified HTTP error: %q", log.String())
	}
}

func TestNotFoundUsesSharedProblemPath(t *testing.T) {
	var log bytes.Buffer
	handler := outcome.NotFound(outcome.Options{Logger: slog.New(slog.NewJSONHandler(&log, nil))})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/missing", nil))
	if response.Code != 404 || !strings.Contains(response.Body.String(), `"code":"not_found"`) {
		t.Fatalf("not found = %d %q", response.Code, response.Body.String())
	}
	if !strings.Contains(log.String(), `"code":"not_found"`) {
		t.Fatalf("log = %q", log.String())
	}
}
