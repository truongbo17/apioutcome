package apioutcome_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	outcome "github.com/truongbo17/apioutcome"
)

func BenchmarkHTTPHandler(b *testing.B) {
	plain := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	wrapped := outcome.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		plain.ServeHTTP(w, r)
		return nil
	}, outcome.Options{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	request := httptest.NewRequest(http.MethodGet, "/", nil)

	for name, handler := range map[string]http.Handler{"plain": plain, "wrapped": wrapped} {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				handler.ServeHTTP(httptest.NewRecorder(), request)
			}
		})
	}
}
