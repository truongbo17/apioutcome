package apioutcome

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"net/http"
)

// DefaultMaxBodyBytes is the size limit used when DecodeJSON receives a
// nonpositive maxBytes value.
const DefaultMaxBodyBytes int64 = 1 << 20

// DecodeJSON reads one JSON value with a hard byte limit and rejects unknown
// fields and trailing values. It returns a public Error on malformed input.
func DecodeJSON(r *http.Request, dst any, maxBytes int64) error {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBodyBytes
	}
	if r == nil || r.Body == nil {
		return Problem(http.StatusBadRequest, "invalid_json", "Invalid JSON body", nil)
	}
	readLimit := maxBytes
	if readLimit < math.MaxInt64 {
		readLimit++
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, readLimit))
	if err != nil {
		return Problem(http.StatusBadRequest, "invalid_json", "Invalid JSON body", err)
	}
	if int64(len(body)) > maxBytes {
		return Problem(http.StatusRequestEntityTooLarge, "body_too_large", "Request body too large", nil)
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return Problem(http.StatusBadRequest, "invalid_json", "Invalid JSON body", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return Problem(http.StatusBadRequest, "invalid_json", "Invalid JSON body", err)
	}
	return nil
}
