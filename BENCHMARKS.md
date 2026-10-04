# Benchmarks

These are measurements of the v0.2 code, not a throughput promise. The benchmark handlers do almost no application work, so the middleware cost is deliberately visible. Each run handles an in-process request: `net/http` and Gin use `httptest.ResponseRecorder`; Fiber uses `app.Test`, which includes request/response conversion. There is no socket, network I/O, database, OTel exporter, or active span. Every instrumented case writes a JSON `slog` record to `io.Discard`. The plain cases do not log. The 422 cases render a problem body; `plain_422` writes only a status, so it is **not** an equivalent error handler.

Environment: Apple M5, macOS 26.2, darwin/amd64 process, Go 1.25.0, one benchmark CPU (`-cpu=1`). Five 200 ms runs per case; the table reports the median `ns/op`, `B/op`, and `allocs/op`. The root and Fiber v3 suites were run separately with the same explicit Go toolchain, and all comparisons below stay within a framework's own baseline.

| Framework | Case | Median ns/op | B/op | allocs/op |
| --- | --- | ---: | ---: | ---: |
| `net/http` | plain 204 | 125.2 | 208 | 4 |
| `net/http` | middleware 204 | 1,377 | 1,072 | 17 |
| `net/http` | `Wrap` 204 | 1,350 | 1,072 | 17 |
| `net/http` | plain status-only 422 | 115.7 | 208 | 4 |
| `net/http` | middleware problem 422 | 2,515 | 2,408 | 35 |
| `net/http` | `Wrap` problem 422 | 2,516 | 2,408 | 35 |
| Gin | plain 204 | 159.4 | 208 | 4 |
| Gin | middleware 204 | 1,374 | 760 | 8 |
| Gin | middleware problem 422 | 2,243 | 1,976 | 21 |
| Fiber v2 | plain 204 | 3,026 | 5,322 | 18 |
| Fiber v2 | middleware 204 | 5,478 | 5,514 | 19 |
| Fiber v2 | middleware problem 422 | 6,960 | 6,372 | 32 |
| Fiber v3 | plain 204 | 2,653 | 5,353 | 19 |
| Fiber v3 | middleware 204 | 5,210 | 5,546 | 20 |
| Fiber v3 | middleware problem 422 | 6,716 | 6,403 | 33 |

The successful middleware path adds about 1.25 µs and 13 allocations to the `net/http` recorder case, 1.21 µs and four allocations to Gin, 2.45 µs and one allocation to Fiber v2, and 2.56 µs and one allocation to Fiber v3 on this machine. The `net/http` wrapper uses `httpsnoop` to preserve optional `ResponseWriter` interfaces; this is a correctness and integration choice with a measurable cost. For real services, measure alongside your router, authentication, logging sink, tracing exporter, and business logic.

## Reproduce

```sh
GOTOOLCHAIN=go1.25.0 go test -run '^$' -bench '^Benchmark(HTTP|Gin|FiberV2)/' -benchmem -benchtime=200ms -count=5 -cpu=1 .
(cd fiberv3outcome && GOTOOLCHAIN=go1.25.0 go test -run '^$' -bench '^BenchmarkFiberV3/' -benchmem -benchtime=200ms -count=5 -cpu=1 .)
```

The source is in [`benchmark_test.go`](benchmark_test.go) and [`fiberv3outcome/benchmark_test.go`](fiberv3outcome/benchmark_test.go). Results will vary with Go version, CPU architecture, runtime load, logger handler, and actual response work. Cross-framework absolute timings are not an application benchmark: the in-process test APIs do different work.
