# apioutcome MVP

## Objective

Give existing Go HTTP APIs one explicit path from returned handler errors to a safe JSON response, a structured log record, and the active OpenTelemetry span. Decode failures must use the same path. The package must never log request bodies or expose internal error causes to clients.

## Scope

- Core `net/http` handler adapter and bounded JSON decoder.
- Optional Gin adapter, sharing error classification and observation.
- RFC 9457 `application/problem+json` error responses with a stable `code` extension.
- Existing `slog.Logger` and active OTel spans; no exporter or global provider setup.
- Explicit behavior when a handler writes a response before returning an error.

Not in MVP: automatic interception of arbitrary framework validation, streaming response support, metrics exporter, Fiber adapter, API schema generation.

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
- `ginoutcome`: optional Gin adapter.
- `examples/`: runnable service.

## Acceptance

Table tests cover malformed/oversized/extra JSON, public and private errors, successful response, committed response, panic behavior, log contents, and span attributes. A Gin integration test covers request validation and error response. README documents migration, limits, and operational behavior. CI runs tests and vet.
