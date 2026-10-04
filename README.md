# apioutcome

**Keep an HTTP failure visible in the response, completion log, and active OpenTelemetry span.**

`apioutcome` is a small Go library for existing backend APIs. Handlers explicitly return errors. The adapter turns an error returned before response commit into an `application/problem+json` response, writes one structured `slog` completion record, and annotates the active OTel span. It does not replace Gin, a validator, a logger, or an OTel exporter.

This is an early `v0` project. The API may change before `v1`.

## Why

Some request failures occur during body decoding or validation, before normal handler logic. If every endpoint maps those errors separately, the HTTP response and telemetry can disagree. `apioutcome` provides one explicit path for those failures. It only observes handlers and routes wired through its adapters; it cannot intercept arbitrary errors from existing framework internals.

## Install

```sh
go get github.com/truongbo17/apioutcome@latest
```

The module currently requires Go 1.25 or later. The root package uses `slog`, OpenTelemetry API, and `httpsnoop`. The `ginoutcome` adapter supports Gin 1.12.

## `net/http` example

```go
logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
opts := apioutcome.Options{Logger: logger}

mux := http.NewServeMux()
mux.Handle("POST /orders", apioutcome.Wrap(func(w http.ResponseWriter, r *http.Request) error {
    var input struct { ProductID string `json:"product_id"` }
    if err := apioutcome.DecodeJSON(r, &input, 64*1024); err != nil {
        return err
    }
    if input.ProductID == "" {
        return apioutcome.Problem(422, "invalid_order", "product_id is required", nil)
    }
    w.WriteHeader(http.StatusCreated)
    return nil
}, opts))
mux.Handle("/", apioutcome.NotFound(opts))
```

Run the complete example with `go run ./examples/orders`, then send:

```sh
curl -i -X POST localhost:8080/orders -d '{"product_id":'
```

The response is HTTP 400 with `code: "invalid_json"`; the completion log carries the same code. If OTel instrumentation has already placed a span in the request context, the span receives `apioutcome.code` and `http.response.status_code`. `apioutcome` does not create a duplicate span or configure an exporter.

## Gin example

```go
router := gin.New()
opts := apioutcome.Options{Logger: logger}
router.POST("/orders", ginoutcome.Wrap(func(c *gin.Context) error {
    var input struct { ProductID string `json:"product_id"` }
    if err := apioutcome.DecodeJSON(c.Request, &input, 64*1024); err != nil {
        return err
    }
    if input.ProductID == "" {
        return apioutcome.Problem(422, "invalid_order", "product_id is required", nil)
    }
    c.Status(http.StatusCreated)
    return nil
}, opts))
router.NoRoute(ginoutcome.NotFound(opts))
```

Both examples require the usual imports for Go, Gin, and `github.com/truongbo17/apioutcome/ginoutcome`.

## Error contract

An unknown error becomes HTTP 500 with `code: "internal_error"`. A public error created by `Problem(status, code, detail, cause)` uses the given status and code. Codes contain only lowercase ASCII letters, digits, and underscores. Invalid status/code values become `internal_error`.

The response uses the RFC 9457 `application/problem+json` fields `type`, `title`, and `status`, plus a `code` extension. `type` is `about:blank`, so `title` is the HTTP status phrase. An optional safe `detail` is included for 4xx errors; 5xx details are discarded. The `code` extension is useful inside an application, but RFC 9457 clients should treat `type` as the primary problem identifier. Do not derive public codes or details from user input or private error messages.

`DecodeJSON` reads at most `maxBytes + 1` bytes, rejects unknown fields and trailing JSON values, and maps malformed input to `invalid_json` and oversized input to `body_too_large`. A nonpositive limit uses 1 MiB. Semantic validation remains your application's responsibility; return a 4xx `Problem` from the handler.

## Operational behavior and limits

- `Wrap` and `ginoutcome.Wrap` convert returned errors and panics only before the response is committed. If the handler already sent bytes or headers, the response stays intact; the completion record marks `error_after_commit: true`.
- A handler that writes a 4xx or 5xx response and returns `nil` is recorded as `unclassified_http_error`. Return a `Problem` before writing for a stable code.
- Logs include method, status, code, duration, commit state, and an existing trace ID. They never include URL paths, query strings, request bodies, private causes, or panic values. OTel spans receive safe status and code attributes; 5xx outcomes are marked as errors.
- To cover unmatched routes, register `NotFound`/`ginoutcome.NotFound` as shown. Middleware and routes outside these adapters are outside the observation boundary.
- Streaming, WebSocket, and hijacked handlers can pass through the `net/http` wrapper, but an error after their first write cannot be turned into a JSON problem response. The `httpsnoop` dependency preserves standard optional `ResponseWriter` interfaces.
- Existing OTel HTTP instrumentation may already set some span attributes. This library annotates that span; configure instrumentation order and sampling in your application.

## Development

```sh
go test ./...
go test -race ./...
go vet ./...
go test -bench=. -run=^$ ./...
```

See [SPEC.md](SPEC.md) for the MVP contract and [SECURITY.md](SECURITY.md) for vulnerability reporting.

## License

MIT. See [LICENSE](LICENSE).
