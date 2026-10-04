# apioutcome

**One request outcome across HTTP response, completion log, and the active trace.**

`apioutcome` is for Go backend APIs that already have a router, logger, and optional OpenTelemetry instrumentation. Add one middleware to observe existing routes. Where an endpoint needs a stable public error code, return or write a `Problem`. The library emits one safe `slog` record per request and annotates the span already in the request context. It does not start a tracer or require a new handler signature.

```go
// Existing net/http router and handlers still work.
handler := apioutcome.Middleware(apioutcome.Options{Logger: logger})(mux)
http.ListenAndServe(":8080", handler)
```

This is a v0 library: review the [contract](SPEC.md) before production use.

## Install and compatibility

| Integration | Package | Tested framework | Go minimum |
| --- | --- | --- | --- |
| `net/http` and compatible routers | `github.com/truongbo17/apioutcome` | Go standard library | 1.22 |
| Gin | `github.com/truongbo17/apioutcome/ginoutcome` | Gin 1.10.1 | 1.22 |
| Fiber v2 | `github.com/truongbo17/apioutcome/fiberoutcome` | Fiber 2.52.15 | 1.22 |
| Fiber v3 | `github.com/truongbo17/apioutcome/fiberv3outcome` | Fiber 3.5.0 | 1.25, separate module |

```sh
go get github.com/truongbo17/apioutcome@v0.2.0
# Only Fiber v3 users need this separate module:
go get github.com/truongbo17/apioutcome/fiberv3outcome@v0.1.0
```

The root module is tested on Go 1.22, 1.24, and 1.26. The Fiber v3 module is tested on Go 1.25 and 1.26. Its separate `go.mod` keeps newer Fiber dependencies out of the root module. Framework versions above are the versions pinned for testing, not a promise that every earlier or later framework release is compatible.

## Adopt in an existing service

### `net/http`, chi, and other `http.Handler` routers

```go
opts := apioutcome.Options{Logger: slog.New(slog.NewJSONHandler(os.Stdout, nil))}
mux := http.NewServeMux()
mux.HandleFunc("POST /orders", func(w http.ResponseWriter, r *http.Request) {
    var order struct { ProductID string `json:"product_id"` }
    if err := apioutcome.DecodeJSON(r, &order, 64*1024); err != nil {
        apioutcome.WriteError(w, r, err)
        return
    }
    if order.ProductID == "" {
        apioutcome.WriteError(w, r, apioutcome.Problem(422, "invalid_order", "product_id is required", nil))
        return
    }
    w.WriteHeader(http.StatusCreated)
})
http.ListenAndServe(":8080", apioutcome.Middleware(opts)(mux))
```

The middleware observes **all** routes, including the router's 404 and handlers you have not changed. `WriteError` is only needed where you want a consistent problem response and code. For a handler that naturally returns an error, `apioutcome.Wrap(handler, opts)` remains available. `WriteError` must run before response bytes or headers are sent to replace the response; install `Middleware` around existing handlers so commit state is tracked.

### Gin

```go
router := gin.New()
router.Use(ginoutcome.Middleware(apioutcome.Options{Logger: logger}))
router.POST("/orders", func(c *gin.Context) {
    if c.Query("product_id") == "" {
        ginoutcome.WriteError(c, apioutcome.Problem(422, "invalid_order", "product_id is required", nil))
        return
    }
    c.Status(http.StatusCreated)
})
```

Existing `gin.HandlerFunc` routes keep their signatures. `ginoutcome.Wrap` is available for returned-error handlers. Put the middleware before routes. Gin's own logger or recovery middleware may produce additional logs or handle panics first depending on order; choose one completion logger and place recovery deliberately.

### Fiber v2

```go
app := fiber.New()
app.Use(fiberoutcome.Middleware(apioutcome.Options{Logger: logger}))
app.Post("/orders", func(c *fiber.Ctx) error {
    if c.Query("product_id") == "" {
        return apioutcome.Problem(422, "invalid_order", "product_id is required", nil)
    }
    return c.SendStatus(http.StatusCreated)
})
```

### Fiber v3

```go
app := fiber.New()
app.Use(fiberv3outcome.Middleware(apioutcome.Options{Logger: logger}))
app.Post("/orders", func(c fiber.Ctx) error {
    if c.Query("product_id") == "" {
        return apioutcome.Problem(422, "invalid_order", "product_id is required", nil)
    }
    return c.SendStatus(http.StatusCreated)
})
```

The Fiber adapters handle returned errors before Fiber's configured `ErrorHandler` runs. If you have a custom `ErrorHandler`, decide which behavior should own error responses and test middleware order. The adapters preserve a body or nondefault status already set before a later error. They do not include or log private error text.

## What the library guarantees

- A `Problem(status, code, detail, cause)` before response commit writes `application/problem+json` with `type: "about:blank"`, the HTTP status phrase as `title`, and a `code` extension. The code is an application constant: lowercase letters, digits, and underscores. Safe `detail` is included only for 4xx; 5xx detail and `cause` stay private.
- Unknown returned errors become HTTP 500 `internal_error`. A handler that directly writes a 4xx or 5xx without calling `WriteError` is logged as `unclassified_http_error`; the library does not reinterpret its body.
- Completion logs include method, status, code, duration, `error_after_commit`, and a trace ID if available. They exclude paths, query strings, request bodies, error causes, and panic values. The active OTel span receives the status and code; the library creates no span or exporter.
- `DecodeJSON` bounds the request body and rejects unknown fields and trailing values. It handles JSON syntax and size, not application validation rules.
- A response already committed stays intact. The completion event flags a later error. Streaming and WebSocket responses can be observed, but cannot be replaced with JSON after their first write.

`about:blank` follows [RFC 9457](https://www.rfc-editor.org/rfc/rfc9457.html); clients should use `type` as the primary problem identifier under that standard. The `code` extension is useful for application-specific handling. Keep public codes and details in trusted source code.

## Where it fits

| Project | Primary job | Use with apioutcome |
| --- | --- | --- |
| [Gin](https://github.com/gin-gonic/gin), [Fiber](https://github.com/gofiber/fiber) | HTTP routing and framework middleware | Yes; use the matching adapter |
| [Huma](https://github.com/danielgtaylor/huma) | API schema, validation, OpenAPI, and HTTP framework | Usually choose Huma's own error flow; apioutcome targets existing routers |
| [otelchi](https://github.com/riandyrn/otelchi) and OTel HTTP instrumentation | Create spans and collect HTTP telemetry | Yes; apioutcome annotates the active span |
| Existing `slog` setup | Own log format and destination | Yes; pass your logger |

The narrow purpose is to keep **the public error, completion record, and active trace in agreement** without replacing the rest of the service. See [BENCHMARKS.md](BENCHMARKS.md) for measured cost and reproducible commands.

## Development

```sh
go test ./...
go test -race ./...
go vet ./...
(cd fiberv3outcome && go test ./... && go vet ./...)
```

See [SPEC.md](SPEC.md), [CONTRIBUTING.md](CONTRIBUTING.md), and [SECURITY.md](SECURITY.md). Licensed under [MIT](LICENSE).
