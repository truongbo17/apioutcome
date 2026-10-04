# apioutcome MVP

## v0.2 integration and compatibility goals

- Go 1.22 is the minimum for the root module. CI tests 1.22, 1.24, and the current stable Go release. Avoid `testing.T.Context` and dependencies whose `go` directive exceeds 1.22 in the root module.
- Existing `net/http` handlers can be observed by wrapping the router once with `Middleware`. No signature change is required. `WriteError` is opt-in where an application wants a stable error code and problem response. `Wrap` remains available for returned errors.
- Existing Gin handlers can be observed with `router.Use(ginoutcome.Middleware(opts))`. `ginoutcome.WriteError` gives a stable code without replacing all handlers. The existing `Wrap` adapter remains available.
- Fiber v2 gets middleware that observes ordinary `fiber.Handler` functions and handles returned errors. A separate Fiber v3 module supports Fiber 3.5.0 without raising the root Go minimum.
- A single request produces one completion event when using one observation middleware. An explicit `WriteError` before response commit aligns response, log, and span code. Unknown framework errors retain safe generic codes.
- The benchmark suite covers plain, middleware success, explicit 4xx, Gin, and Fiber, with the same logger sink and in-process request setup. Publish Go version, OS/arch, CPU, `-count`, ns/op, B/op, allocs/op, and limitations. Do not compare numbers from different Go versions as if they were controlled A/B tests.

## Objective

Give existing Go HTTP APIs one explicit path from handler errors to a safe JSON response, a structured log record, and the active OpenTelemetry span. Existing handlers remain usable through middleware; returned-error handlers are optional. Decode failures must use the same path. The package must never log request bodies or expose internal error causes to clients.

## Scope

- Core `net/http` middleware, returned-error adapter, and bounded JSON decoder.
- Gin, Fiber v2, and separately versioned Fiber v3 adapters, sharing error classification and observation.
- RFC 9457 `application/problem+json` error responses with a stable `code` extension.
- Existing `slog.Logger` and active OTel spans; no exporter or global provider setup.
- Explicit behavior when a handler writes a response before returning an error.

Outside scope: automatic interception of arbitrary framework validation, streaming response rewriting, metrics exporter, API schema generation.

## Contract

- `Problem(status, code, detail, cause)` creates a public error; its cause stays private. With `about:blank`, `title` is the HTTP status phrase and safe detail is optional.
- Unknown errors become HTTP 500 `internal_error` with a generic title.
- `DecodeJSON` accepts exactly one JSON value, enforces a size limit, and maps malformed input to `invalid_json`.
- Handlers returning errors before a response is committed receive a problem response. If already committed, the package preserves the response and records the error.
- Exactly one completion log per wrapped request. Logs contain a safe code, never the private cause, request body, or URL query. Applications may separately log a vetted cause.
- The active span gets status and error attributes. The package does not create a second span.

## Commands

Run tests: `go test ./...`
Race tests: `go test -race ./...`
Static checks: `go vet ./...`
Formatting: `gofmt -w .`

## Structure

- Root package: errors, decoding, net/http adapter, observation.
- `ginoutcome`: Gin adapter.
- `fiberoutcome`: Fiber v2 adapter.
- `fiberv3outcome`: separate Fiber v3 module.
- `examples/`: runnable service.

## Acceptance

Table tests cover malformed/oversized/extra JSON, public and private errors, successful response, committed response, panic behavior, log contents, and span attributes. A Gin integration test covers request validation and error response. README documents migration, limits, and operational behavior. CI runs tests and vet.
