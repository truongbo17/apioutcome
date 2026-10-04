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
