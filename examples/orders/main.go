package main

import (
	"encoding/json"
	"log"
	"log/slog"
	"net/http"
	"os"

	outcome "github.com/truongbo17/apioutcome"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	opts := outcome.Options{Logger: logger}
	mux := http.NewServeMux()
	mux.Handle("POST /orders", outcome.Wrap(createOrder, opts))
	mux.Handle("/", outcome.NotFound(opts))
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func createOrder(w http.ResponseWriter, r *http.Request) error {
	var input struct {
		ProductID string `json:"product_id"`
	}
	if err := outcome.DecodeJSON(r, &input, 64*1024); err != nil {
		return err
	}
	if input.ProductID == "" {
		return outcome.Problem(http.StatusUnprocessableEntity, "invalid_order", "product_id is required", nil)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	return json.NewEncoder(w).Encode(map[string]string{"id": "ord_123"})
}
