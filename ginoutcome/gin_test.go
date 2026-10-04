package ginoutcome_test

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	outcome "github.com/truongbo17/apioutcome"
	"github.com/truongbo17/apioutcome/ginoutcome"
)

func TestGinDecodeFailureAndNoRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var log bytes.Buffer
	opts := outcome.Options{Logger: slog.New(slog.NewJSONHandler(&log, nil))}
	router := gin.New()
	router.POST("/orders", ginoutcome.Wrap(func(c *gin.Context) error {
		var input struct {
			Name string `json:"name"`
		}
		return outcome.DecodeJSON(c.Request, &input, 100)
	}, opts))
	router.NoRoute(ginoutcome.NotFound(opts))

	for _, tc := range []struct {
		method, path, body string
		status             int
		code               string
	}{
		{http.MethodPost, "/orders", `{"name":`, 400, "invalid_json"},
		{http.MethodGet, "/missing", "", 404, "not_found"},
	} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))
		if response.Code != tc.status || !strings.Contains(response.Body.String(), `"code":"`+tc.code+`"`) {
			t.Fatalf("response = %d %q", response.Code, response.Body.String())
		}
	}
	if strings.Count(strings.TrimSpace(log.String()), "\n") != 1 {
		t.Fatalf("want two completion records, got %q", log.String())
	}
}

func TestMiddlewareKeepsExistingGinHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var log bytes.Buffer
	opts := outcome.Options{Logger: slog.New(slog.NewJSONHandler(&log, nil))}
	router := gin.New()
	router.Use(ginoutcome.Middleware(opts))
	router.POST("/orders", func(c *gin.Context) {
		ginoutcome.WriteError(c, outcome.Problem(422, "invalid_order", "Missing product", nil))
	})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/orders", nil))
	if response.Code != 422 || !strings.Contains(response.Body.String(), `"code":"invalid_order"`) || !strings.Contains(log.String(), `"code":"invalid_order"`) {
		t.Fatalf("response/log = %d %q %q", response.Code, response.Body.String(), log.String())
	}
	if strings.Count(log.String(), "\n") != 1 {
		t.Fatalf("expected one log: %q", log.String())
	}
}

func TestGinMiddlewareObservesUnmatchedRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var log bytes.Buffer
	router := gin.New()
	router.Use(ginoutcome.Middleware(outcome.Options{Logger: slog.New(slog.NewJSONHandler(&log, nil))}))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/missing", nil))
	if response.Code != 404 || !strings.Contains(log.String(), `"code":"unclassified_http_error"`) {
		t.Fatalf("response/log = %d %q", response.Code, log.String())
	}
}
